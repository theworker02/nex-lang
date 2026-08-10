/**
 * TypeScript Nexus web host — HTTP, templates, static files, demo DB.
 * Mirrors the Go pkg/host surface enough to run nex-registry/app/*.nex.
 */

import * as crypto from "crypto";
import * as fs from "fs";
import * as http from "http";
import * as path from "path";
import { parse as parseToml } from "smol-toml";
import { BuiltinObj, ApplyUserFnAsync } from "../language/builtins";
import { Environment, FunctionObj } from "../language/evaluator";
import {
  ErrorObj,
  FALSE_OBJ,
  HashObj,
  IntegerObj,
  NexusObject,
  NULL_OBJ,
  StringObj,
  TRUE_OBJ,
} from "../language/values";
import {
  asBool,
  asInt,
  asString,
  expectArgs,
  expectMinArgs,
  fromJs,
  hashGetString,
  toJs,
} from "./convert";
import {
  nxdKitScript,
  renderDesignDocument,
  renderDesignTree,
  themeFromData,
  themeToCss,
} from "./design";
import { DOCS_PAGES } from "./docs";
import { MemoryDB } from "./db_memory";
import { PostgresDB } from "./db_postgres";
import { markdownToHtml } from "./markdown";
import {
  HttpMethod,
  IncomingRequest,
  Router,
} from "./router";
import {
  loadGoTemplates,
  normalizeTemplateData,
  TemplateEngine,
} from "./templates";

export interface WebHostConfig {
  storageDir: string;
  baseUrl: string;
  cdnBaseUrl: string;
  listenAddr: string;
  maxUploadBytes: number;
  appDir: string;
  webDir: string;
  databaseUrl?: string;
  migrationsDir?: string;
}

/**
 * Either backend satisfies the read paths the host needs; Postgres-only
 * capabilities (auth tokens, api keys, orgs, owners) are reached through
 * `isPostgres()` narrowing at each call site.
 */
export type RegistryDB = MemoryDB | PostgresDB;

export function isPostgres(db: RegistryDB): db is PostgresDB {
  return (db as PostgresDB).mode === "postgres";
}

export interface WebHost {
  readonly config: WebHostConfig;
  readonly router: Router;
  readonly db: RegistryDB;
  readonly dbMode: "memory" | "postgres";
  routeCount: number;
  install(env: Environment, apply: ApplyUserFnAsync): void;
  listen(): Promise<void>;
  close(): Promise<void>;
}

const SESSION_COOKIE = "nex_session";
const SESSION_TTL_SEC = 30 * 24 * 60 * 60;

function cookieSecureFlag(baseUrl: string): boolean {
  if (process.env.COOKIE_SECURE === "1" || process.env.COOKIE_SECURE === "true") {
    return true;
  }
  if (process.env.COOKIE_SECURE === "0" || process.env.COOKIE_SECURE === "false") {
    return false;
  }
  return baseUrl.toLowerCase().startsWith("https://");
}

function parseListenAddr(addr: string): { host: string; port: number } {
  const trimmed = addr.trim();
  if (trimmed.startsWith(":")) {
    return { host: "0.0.0.0", port: Number.parseInt(trimmed.slice(1), 10) || 8080 };
  }
  const idx = trimmed.lastIndexOf(":");
  if (idx > 0) {
    return {
      host: trimmed.slice(0, idx),
      port: Number.parseInt(trimmed.slice(idx + 1), 10) || 8080,
    };
  }
  return { host: "0.0.0.0", port: Number.parseInt(trimmed, 10) || 8080 };
}

function parseCookies(header: string | undefined): Record<string, string> {
  const out: Record<string, string> = {};
  if (!header) {
    return out;
  }
  for (const part of header.split(";")) {
    const eq = part.indexOf("=");
    if (eq < 0) {
      continue;
    }
    const k = part.slice(0, eq).trim();
    const v = part.slice(eq + 1).trim();
    out[k] = decodeURIComponent(v);
  }
  return out;
}

function parseQuery(url: URL): Record<string, string> {
  const out: Record<string, string> = {};
  url.searchParams.forEach((v, k) => {
    if (!(k in out)) {
      out[k] = v;
    }
  });
  return out;
}

function parseFormUrlEncoded(body: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const part of body.split("&")) {
    if (!part) {
      continue;
    }
    const eq = part.indexOf("=");
    const k = decodeURIComponent((eq < 0 ? part : part.slice(0, eq)).replace(/\+/g, " "));
    const v = decodeURIComponent((eq < 0 ? "" : part.slice(eq + 1)).replace(/\+/g, " "));
    out[k] = v;
  }
  return out;
}

interface MultipartResult {
  fields: Record<string, string>;
  files: Record<
    string,
    { filename: string; data: Buffer; contentType: string }
  >;
}

function parseMultipart(body: Buffer, boundary: string): MultipartResult {
  const fields: Record<string, string> = {};
  const files: MultipartResult["files"] = {};
  const sep = Buffer.from(`--${boundary}`);
  let start = body.indexOf(sep) + sep.length;
  while (start < body.length) {
    if (body[start] === 45 && body[start + 1] === 45) {
      break; // --
    }
    if (body[start] === 13 && body[start + 1] === 10) {
      start += 2;
    }
    const next = body.indexOf(sep, start);
    const chunk = body.subarray(start, next < 0 ? body.length : next - 2);
    const headerEnd = chunk.indexOf("\r\n\r\n");
    if (headerEnd < 0) {
      break;
    }
    const headerText = chunk.subarray(0, headerEnd).toString("utf8");
    const content = chunk.subarray(headerEnd + 4);
    const nameMatch = /name="([^"]+)"/.exec(headerText);
    const fileMatch = /filename="([^"]*)"/.exec(headerText);
    const ctMatch = /Content-Type:\s*([^\r\n]+)/i.exec(headerText);
    if (nameMatch) {
      const name = nameMatch[1]!;
      if (fileMatch) {
        files[name] = {
          filename: fileMatch[1] || "upload",
          data: Buffer.from(content),
          contentType: ctMatch?.[1]?.trim() || "application/octet-stream",
        };
      } else {
        fields[name] = content.toString("utf8");
      }
    }
    if (next < 0) {
      break;
    }
    start = next + sep.length;
  }
  return { fields, files };
}

function loadTemplatesFromWebDir(webDir: string): TemplateEngine | null {
  const tplDir = path.join(webDir, "templates");
  if (!fs.existsSync(tplDir)) {
    return null;
  }
  const basePath = path.join(tplDir, "base.html");
  if (!fs.existsSync(basePath)) {
    return null;
  }
  const base = fs.readFileSync(basePath, "utf8");
  const pages: Record<string, string> = {};
  for (const name of fs.readdirSync(tplDir)) {
    if (!name.endsWith(".html") || name === "base.html") {
      continue;
    }
    const full = path.join(tplDir, name);
    if (fs.statSync(full).isFile()) {
      pages[name] = fs.readFileSync(full, "utf8");
    }
  }
  const partials: Record<string, string> = {};
  const partialDir = path.join(tplDir, "partials");
  if (fs.existsSync(partialDir)) {
    for (const name of fs.readdirSync(partialDir)) {
      if (!name.endsWith(".html")) {
        continue;
      }
      partials[name] = fs.readFileSync(path.join(partialDir, name), "utf8");
    }
  }
  return loadGoTemplates({ base, pages, partials });
}

function pbkdf2Hash(password: string): string {
  const salt = crypto.randomBytes(16).toString("hex");
  const hash = crypto.pbkdf2Sync(password, salt, 100_000, 32, "sha256").toString("hex");
  return `pbkdf2$${salt}$${hash}`;
}

function pbkdf2Check(stored: string, password: string): boolean {
  if (stored.startsWith("pbkdf2$")) {
    const [, salt, hash] = stored.split("$");
    if (!salt || !hash) {
      return false;
    }
    const check = crypto.pbkdf2Sync(password, salt, 100_000, 32, "sha256").toString("hex");
    return crypto.timingSafeEqual(Buffer.from(hash, "hex"), Buffer.from(check, "hex"));
  }
  // bcrypt hashes from Go host — accept only exact demo match
  return false;
}

export function resolveWebHostConfig(
  entryFile: string,
  cwd = process.cwd(),
): WebHostConfig {
  const absFile = path.resolve(entryFile);
  const appDir = path.resolve(process.env.NEX_APP_DIR || path.dirname(absFile));
  let webDir = process.env.NEX_WEB_DIR || "";
  if (!webDir) {
    const candidates = [
      path.join(cwd, "web"),
      path.join(path.dirname(appDir), "web"),
      path.join(appDir, "..", "web"),
    ];
    for (const c of candidates) {
      const abs = path.resolve(c);
      if (fs.existsSync(path.join(abs, "templates"))) {
        webDir = abs;
        break;
      }
    }
  } else {
    webDir = path.resolve(webDir);
  }

  const storageDir = path.resolve(process.env.STORAGE_DIR || path.join(path.dirname(appDir), "storage"));
  const listenAddr = process.env.LISTEN_ADDR || ":8080";
  const baseUrl = (process.env.BASE_URL || "http://localhost:8080").replace(/\/$/, "");
  const cdnBaseUrl = (process.env.CDN_BASE_URL || baseUrl).replace(/\/$/, "");
  const maxUploadBytes = Number.parseInt(process.env.MAX_UPLOAD_BYTES || "", 10) || 64 << 20;
  const databaseUrl = (process.env.DATABASE_URL || "").trim() || undefined;
  const migrationsDir =
    process.env.MIGRATIONS_DIR ||
    path.join(path.dirname(appDir), "migrations");

  return {
    storageDir,
    baseUrl,
    cdnBaseUrl,
    listenAddr,
    maxUploadBytes,
    appDir,
    webDir,
    databaseUrl,
    migrationsDir,
  };
}

export function createWebHost(config: WebHostConfig): WebHost {
  const router = new Router();
  let db: RegistryDB;
  let dbMode: "memory" | "postgres";
  if (config.databaseUrl) {
    try {
      const pg = new PostgresDB(config.databaseUrl);
      if (config.migrationsDir) {
        pg.applyMigrations(config.migrationsDir);
      }
      pg.seedFromStorage(config.storageDir);
      db = pg;
      dbMode = "postgres";
      // eslint-disable-next-line no-console
      console.log(
        `[host] Postgres DB (${pg.packages.length} packages, ${pg.versions.length} versions)`,
      );
    } catch (e) {
      // Fall back to the in-memory store rather than serving a broken site.
      // eslint-disable-next-line no-console
      console.error(
        `[host] Postgres init failed (${e instanceof Error ? e.message : String(e)}); falling back to in-memory demo DB`,
      );
      const mem = new MemoryDB();
      mem.seedFromStorage(config.storageDir);
      db = mem;
      dbMode = "memory";
    }
  } else {
    const mem = new MemoryDB();
    mem.seedFromStorage(config.storageDir);
    db = mem;
    dbMode = "memory";
    // eslint-disable-next-line no-console
    console.log("[host] in-memory demo DB (seeded from STORAGE_DIR)");
  }
  const secureCookies = cookieSecureFlag(config.baseUrl);
  const forms = new Map<string, MultipartResult>();
  let templates = config.webDir ? loadTemplatesFromWebDir(config.webDir) : null;
  let server: http.Server | null = null;
  let applyFn: ApplyUserFnAsync = () => new ErrorObj("host applyFunction not set");

  const host: WebHost = {
    config,
    router,
    db,
    dbMode,
    get routeCount() {
      return router.routeCount;
    },
    install(env: Environment, apply: ApplyUserFnAsync) {
      applyFn = apply;
      installBuiltins(env);
    },
    async listen() {
      await listenAndServe();
    },
    async close() {
      await new Promise<void>((resolve, reject) => {
        if (!server) {
          resolve();
          return;
        }
        server.close((err) => (err ? reject(err) : resolve()));
      });
      if (isPostgres(db)) {
        db.close();
      }
    },
  };

  function setBuiltin(
    env: Environment,
    name: string,
    fn: (...args: NexusObject[]) => NexusObject,
  ): void {
    env.set(name, new BuiltinObj(fn));
  }

  function routeBuiltin(method: HttpMethod) {
    return (...args: NexusObject[]): NexusObject => {
      const err = expectArgs(`http_${method.toLowerCase()}`, 2, args);
      if (err) {
        return err;
      }
      const routePath = asString(args[0]);
      const fn = args[1];
      if (!routePath || !(fn instanceof FunctionObj)) {
        return new ErrorObj(`http_${method.toLowerCase()} expects (path, function)`);
      }
      router.add(method, routePath, async (req) => {
        const reqObj = requestToHash(req);
        return applyFn(fn, [reqObj]);
      });
      return NULL_OBJ;
    };
  }

  function requestToHash(req: IncomingRequest): HashObj {
    const h = new HashObj();
    h.setString("method", new StringObj(req.method));
    h.setString("path", new StringObj(req.path));
    h.setString("request_id", new StringObj(req.requestId));
    h.setString("auth_via", new StringObj(req.authVia));
    h.setString("query", fromJs(req.query));
    h.setString("headers", fromJs(req.headers));
    h.setString("params", fromJs(req.params));
    h.setString("cookies", fromJs(req.cookies));
    h.setString("form", fromJs(req.form));
    h.setString("body", new StringObj(req.body));
    h.setString("user", req.user ? fromJs(req.user) : NULL_OBJ);
    h.setString("api_key_scope", new StringObj(""));
    h.setString("api_key_id", NULL_OBJ);
    return h;
  }

  function writeResponse(
    res: http.ServerResponse,
    _req: http.IncomingMessage,
    result: NexusObject,
    user: Record<string, unknown> | null,
  ): void {
    if (!(result instanceof HashObj)) {
      res.statusCode = 200;
      res.setHeader("Content-Type", "application/json; charset=utf-8");
      res.end(JSON.stringify({ result: toJs(result) }));
      return;
    }

    const setCookie = result.getString("set_cookie");
    if (setCookie instanceof HashObj) {
      const name = hashGetString(setCookie, "name");
      const value = hashGetString(setCookie, "value");
      const maxAge = asInt(setCookie.getString("max_age")) ?? SESSION_TTL_SEC;
      const secure = secureCookies ? "; Secure" : "";
      res.setHeader(
        "Set-Cookie",
        `${name}=${encodeURIComponent(value)}; Path=/; HttpOnly; SameSite=Lax; Max-Age=${maxAge}${secure}`,
      );
    }
    const clearCookie = result.getString("clear_cookie");
    if (clearCookie instanceof StringObj) {
      const secure = secureCookies ? "; Secure" : "";
      res.setHeader(
        "Set-Cookie",
        `${clearCookie.value}=; Path=/; HttpOnly; Max-Age=0${secure}`,
      );
    }

    const status = asInt(result.getString("status")) ?? 200;

    const redir = result.getString("redirect");
    if (redir instanceof StringObj) {
      res.statusCode = status;
      res.setHeader("Location", redir.value);
      res.end();
      return;
    }

    const file = result.getString("file");
    if (file instanceof StringObj) {
      const filename = hashGetString(result, "filename");
      const ct = hashGetString(result, "content_type") || "application/octet-stream";
      if (!fs.existsSync(file.value)) {
        res.statusCode = 404;
        res.setHeader("Content-Type", "application/json");
        res.end(JSON.stringify({ error: "file not found" }));
        return;
      }
      res.statusCode = status;
      res.setHeader("Content-Type", ct);
      if (filename) {
        res.setHeader("Content-Disposition", `attachment; filename="${filename}"`);
      }
      fs.createReadStream(file.value).pipe(res);
      return;
    }

    const htmlPage = result.getString("html");
    if (htmlPage instanceof StringObj) {
      const dataRaw = result.getString("data");
      const data =
        dataRaw instanceof HashObj
          ? (toJs(dataRaw) as Record<string, unknown>)
          : {};
      if (user) {
        data.CurrentUser = user;
      }
      data.GitHubAuthEnabled = Boolean(
        process.env.GITHUB_CLIENT_ID && process.env.GITHUB_CLIENT_SECRET,
      );
      try {
        if (!templates && config.webDir) {
          templates = loadTemplatesFromWebDir(config.webDir);
        }
        if (!templates) {
          throw new Error("templates not loaded (set NEX_WEB_DIR)");
        }
        const body = templates.render(
          htmlPage.value,
          normalizeTemplateData(data),
        );
        res.statusCode = status;
        res.setHeader("Content-Type", "text/html; charset=utf-8");
        res.end(body);
      } catch (err) {
        // eslint-disable-next-line no-console
        console.error("template error", err);
        res.statusCode = 500;
        res.end("template error");
      }
      return;
    }

    const jsonPayload = result.getString("json");
    if (jsonPayload.type !== "NULL") {
      res.statusCode = status;
      res.setHeader("Content-Type", "application/json; charset=utf-8");
      res.end(JSON.stringify(toJs(jsonPayload)));
      return;
    }

    const body = result.getString("body");
    if (body.type !== "NULL") {
      const ct = hashGetString(result, "content_type") || "text/plain; charset=utf-8";
      res.statusCode = status;
      res.setHeader("Content-Type", ct);
      const text =
        body instanceof StringObj ? body.value : body.inspect();
      res.end(text);
      return;
    }

    res.statusCode = status;
    res.setHeader("Content-Type", "application/json; charset=utf-8");
    res.end(JSON.stringify(toJs(result)));
  }

  function resolveUser(req: http.IncomingMessage): {
    user: Record<string, unknown> | null;
    via: string;
  } {
    const auth = req.headers.authorization;
    if (auth && auth.toLowerCase().startsWith("bearer ")) {
      const token = auth.slice(7).trim();
      const u = db.userFromSession(token);
      if (u) {
        return { user: u as unknown as Record<string, unknown>, via: "bearer" };
      }
    }
    const cookies = parseCookies(req.headers.cookie);
    const sess = cookies[SESSION_COOKIE];
    if (sess) {
      const u = db.userFromSession(sess);
      if (u) {
        return { user: u as unknown as Record<string, unknown>, via: "cookie" };
      }
    }
    return { user: null, via: "" };
  }

  function installBuiltins(env: Environment): void {
    setBuiltin(env, "env", (...args) => {
      const err = expectArgs("env", 1, args);
      if (err) {
        return err;
      }
      const key = asString(args[0]);
      if (!key) {
        return new ErrorObj("env expects string");
      }
      return new StringObj(process.env[key] ?? "");
    });

    setBuiltin(env, "config", () =>
      fromJs({
        storage_dir: config.storageDir,
        base_url: config.baseUrl,
        cdn_base_url: config.cdnBaseUrl,
        listen_addr: config.listenAddr,
        max_upload_bytes: config.maxUploadBytes,
        app_dir: config.appDir,
        web_dir: config.webDir,
        mode: dbMode,
        db_mode: dbMode,
        database_url_set: Boolean(config.databaseUrl),
      }),
    );

    setBuiltin(env, "http_get", routeBuiltin("GET"));
    setBuiltin(env, "http_post", routeBuiltin("POST"));
    setBuiltin(env, "http_put", routeBuiltin("PUT"));
    setBuiltin(env, "http_patch", routeBuiltin("PATCH"));
    setBuiltin(env, "http_delete", routeBuiltin("DELETE"));
    setBuiltin(env, "http_not_found", (...args) => {
      const err = expectArgs("http_not_found", 1, args);
      if (err) {
        return err;
      }
      const fn = args[0];
      if (!(fn instanceof FunctionObj)) {
        return new ErrorObj("http_not_found expects function");
      }
      router.notFoundHandler = async (req) => applyFn(fn, [requestToHash(req)]);
      return NULL_OBJ;
    });

    setBuiltin(env, "json", (...args) => {
      const err = expectMinArgs("json", 1, args);
      if (err) {
        return err;
      }
      let status = 200;
      let payload = args[0]!;
      if (args.length === 2) {
        const s = asInt(args[0]);
        if (s !== null) {
          status = s;
          payload = args[1]!;
        }
      }
      const out = new HashObj();
      out.setString("status", new IntegerObj(status));
      out.setString("json", payload);
      return out;
    });

    setBuiltin(env, "html", (...args) => {
      const err = expectMinArgs("html", 2, args);
      if (err) {
        return err;
      }
      let status = 200;
      let pageIdx = 0;
      let dataIdx = 1;
      if (args.length === 3) {
        const s = asInt(args[0]);
        if (s !== null) {
          status = s;
          pageIdx = 1;
          dataIdx = 2;
        }
      }
      const page = asString(args[pageIdx]);
      if (!page) {
        return new ErrorObj("html page must be string");
      }
      const out = new HashObj();
      out.setString("status", new IntegerObj(status));
      out.setString("html", new StringObj(page));
      out.setString("data", args[dataIdx]!);
      return out;
    });

    /** Return a raw HTML document body (no Go templates). */
    setBuiltin(env, "html_doc", (...args) => {
      const err = expectMinArgs("html_doc", 1, args);
      if (err) {
        return err;
      }
      let status = 200;
      let bodyIdx = 0;
      if (args.length >= 2) {
        const s = asInt(args[0]);
        if (s !== null) {
          status = s;
          bodyIdx = 1;
        }
      }
      const body = asString(args[bodyIdx]);
      if (body === null) {
        return new ErrorObj("html_doc expects an HTML string");
      }
      const out = new HashObj();
      out.setString("status", new IntegerObj(status));
      out.setString("body", new StringObj(body));
      out.setString("content_type", new StringObj("text/html; charset=utf-8"));
      return out;
    });

    /** Render a design tree (hash) to an HTML fragment string. */
    setBuiltin(env, "design_render", (...args) => {
      const err = expectMinArgs("design_render", 1, args);
      if (err) {
        return err;
      }
      const tree = toJs(args[0]!);
      const theme = args.length >= 2 ? toJs(args[1]!) : undefined;
      try {
        return new StringObj(renderDesignTree(tree, theme));
      } catch (e) {
        return new ErrorObj(
          `design_render: ${e instanceof Error ? e.message : String(e)}`,
        );
      }
    });

    /** Render a design page tree to a full HTML document string. */
    setBuiltin(env, "design_document", (...args) => {
      const err = expectArgs("design_document", 1, args);
      if (err) {
        return err;
      }
      try {
        return new StringObj(renderDesignDocument(toJs(args[0]!)));
      } catch (e) {
        return new ErrorObj(
          `design_document: ${e instanceof Error ? e.message : String(e)}`,
        );
      }
    });

    /** HTTP response for a design page (full document). */
    setBuiltin(env, "design_response", (...args) => {
      const err = expectMinArgs("design_response", 1, args);
      if (err) {
        return err;
      }
      let status = 200;
      let pageIdx = 0;
      if (args.length >= 2) {
        const s = asInt(args[0]);
        if (s !== null) {
          status = s;
          pageIdx = 1;
        }
      }
      try {
        const doc = renderDesignDocument(toJs(args[pageIdx]!));
        const out = new HashObj();
        out.setString("status", new IntegerObj(status));
        out.setString("body", new StringObj(doc));
        out.setString("content_type", new StringObj("text/html; charset=utf-8"));
        return out;
      } catch (e) {
        return new ErrorObj(
          `design_response: ${e instanceof Error ? e.message : String(e)}`,
        );
      }
    });

    /** Emit CSS custom properties + structural rules for a theme. */
    setBuiltin(env, "design_css", (...args) => {
      const err = expectArgs("design_css", 1, args);
      if (err) {
        return err;
      }
      try {
        return new StringObj(themeToCss(themeFromData(toJs(args[0]!))));
      } catch (e) {
        return new ErrorObj(
          `design_css: ${e instanceof Error ? e.message : String(e)}`,
        );
      }
    });

    setBuiltin(env, "redirect", (...args) => {
      const err = expectArgs("redirect", 1, args);
      if (err) {
        return err;
      }
      const url = asString(args[0]);
      if (!url) {
        return new ErrorObj("redirect expects string");
      }
      const out = new HashObj();
      out.setString("status", new IntegerObj(303));
      out.setString("redirect", new StringObj(url));
      return out;
    });

    setBuiltin(env, "file_response", (...args) => {
      const err = expectMinArgs("file_response", 2, args);
      if (err) {
        return err;
      }
      const filePath = asString(args[0]);
      const name = asString(args[1]);
      if (!filePath || !name) {
        return new ErrorObj("file_response expects (path, filename [, content_type])");
      }
      const ct = asString(args[2]) || "application/octet-stream";
      const out = new HashObj();
      out.setString("status", new IntegerObj(200));
      out.setString("file", new StringObj(filePath));
      out.setString("filename", new StringObj(name));
      out.setString("content_type", new StringObj(ct));
      return out;
    });

    setBuiltin(env, "with_cookie", (...args) => {
      const err = expectArgs("with_cookie", 3, args);
      if (err) {
        return err;
      }
      const resp = args[0];
      if (!(resp instanceof HashObj)) {
        return new ErrorObj("with_cookie expects response hash");
      }
      const name = asString(args[1]);
      const value = asString(args[2]);
      if (!name || value === null) {
        return new ErrorObj("cookie name/value must be strings");
      }
      const cookie = new HashObj();
      cookie.setString("name", new StringObj(name));
      cookie.setString("value", new StringObj(value));
      cookie.setString("max_age", new IntegerObj(SESSION_TTL_SEC));
      resp.setString("set_cookie", cookie);
      return resp;
    });

    setBuiltin(env, "clear_cookie", (...args) => {
      const err = expectArgs("clear_cookie", 2, args);
      if (err) {
        return err;
      }
      const resp = args[0];
      if (!(resp instanceof HashObj)) {
        return new ErrorObj("clear_cookie expects response hash");
      }
      const name = asString(args[1]);
      if (!name) {
        return new ErrorObj("cookie name must be string");
      }
      resp.setString("clear_cookie", new StringObj(name));
      return resp;
    });

    setBuiltin(env, "json_parse", (...args) => {
      const err = expectArgs("json_parse", 1, args);
      if (err) {
        return err;
      }
      const s = asString(args[0]);
      if (s === null) {
        return new ErrorObj("json_parse expects string");
      }
      try {
        return fromJs(JSON.parse(s));
      } catch (e) {
        return new ErrorObj(`json_parse: ${e instanceof Error ? e.message : String(e)}`);
      }
    });

    setBuiltin(env, "json_stringify", (...args) => {
      const err = expectArgs("json_stringify", 1, args);
      if (err) {
        return err;
      }
      try {
        return new StringObj(JSON.stringify(toJs(args[0]!)));
      } catch (e) {
        return new ErrorObj(`json_stringify: ${e instanceof Error ? e.message : String(e)}`);
      }
    });

    setBuiltin(env, "toml_parse", (...args) => {
      const err = expectArgs("toml_parse", 1, args);
      if (err) {
        return err;
      }
      const s = asString(args[0]);
      if (s === null) {
        return new ErrorObj("toml_parse expects string");
      }
      try {
        return fromJs(parseToml(s));
      } catch (e) {
        return new ErrorObj(`toml_parse: ${e instanceof Error ? e.message : String(e)}`);
      }
    });

    setBuiltin(env, "sha256", (...args) => {
      const err = expectArgs("sha256", 1, args);
      if (err) {
        return err;
      }
      const s = asString(args[0]);
      if (s === null) {
        return new ErrorObj("sha256 expects string");
      }
      return new StringObj(crypto.createHash("sha256").update(s).digest("hex"));
    });

    setBuiltin(env, "sha256_bytes", (...args) => {
      const err = expectArgs("sha256_bytes", 1, args);
      if (err) {
        return err;
      }
      const s = asString(args[0]);
      if (s === null) {
        return new ErrorObj("sha256_bytes expects string");
      }
      return new StringObj(
        "sha256:" + crypto.createHash("sha256").update(s).digest("hex"),
      );
    });

    setBuiltin(env, "bcrypt_hash", (...args) => {
      const err = expectArgs("bcrypt_hash", 1, args);
      if (err) {
        return err;
      }
      const s = asString(args[0]);
      if (s === null) {
        return new ErrorObj("bcrypt_hash expects string");
      }
      return new StringObj(pbkdf2Hash(s));
    });

    setBuiltin(env, "bcrypt_check", (...args) => {
      const err = expectArgs("bcrypt_check", 2, args);
      if (err) {
        return err;
      }
      const hash = asString(args[0]);
      const pw = asString(args[1]);
      if (hash === null || pw === null) {
        return new ErrorObj("bcrypt_check expects (hash, password)");
      }
      return pbkdf2Check(hash, pw) ? TRUE_OBJ : FALSE_OBJ;
    });

    setBuiltin(env, "random_hex", (...args) => {
      let n = 32;
      if (args.length === 1) {
        n = asInt(args[0]) ?? 32;
      }
      return new StringObj(crypto.randomBytes(n).toString("hex"));
    });

    setBuiltin(env, "gravatar_url", (...args) => {
      const err = expectArgs("gravatar_url", 1, args);
      if (err) {
        return err;
      }
      const email = (asString(args[0]) || "").trim().toLowerCase();
      const sum = crypto.createHash("sha256").update(email).digest("hex");
      return new StringObj(
        `https://www.gravatar.com/avatar/${sum}?d=identicon&s=160`,
      );
    });

    setBuiltin(env, "path_join", (...args) => {
      const parts: string[] = [];
      for (const a of args) {
        const s = asString(a);
        if (s === null) {
          return new ErrorObj("path_join expects strings");
        }
        parts.push(s);
      }
      return new StringObj(path.join(...parts));
    });

    setBuiltin(env, "re_match", (...args) => {
      const err = expectArgs("re_match", 2, args);
      if (err) {
        return err;
      }
      const pat = asString(args[0]);
      const s = asString(args[1]);
      if (pat === null || s === null) {
        return new ErrorObj("re_match expects (pattern, string)");
      }
      try {
        return new RegExp(pat).test(s) ? TRUE_OBJ : FALSE_OBJ;
      } catch (e) {
        return new ErrorObj(e instanceof Error ? e.message : String(e));
      }
    });

    setBuiltin(env, "markdown_html", (...args) => {
      const err = expectArgs("markdown_html", 1, args);
      if (err) {
        return err;
      }
      const src = asString(args[0]);
      if (src === null) {
        return new ErrorObj("markdown_html expects string");
      }
      return new StringObj(markdownToHtml(src));
    });

    setBuiltin(env, "docs_get", (...args) => {
      let id = "overview";
      if (args.length > 0) {
        const s = asString(args[0]);
        if (s) {
          id = s;
        }
      }
      const page = DOCS_PAGES[id];
      if (!page) {
        return NULL_OBJ;
      }
      return fromJs({
        id,
        Title: page.Title,
        Lead: page.Lead,
        Section: page.Section,
        Body: page.Body,
      });
    });

    setBuiltin(env, "multipart_text", (...args) => {
      const err = expectArgs("multipart_text", 2, args);
      if (err) {
        return err;
      }
      const reqHash = args[0];
      const field = asString(args[1]);
      if (!(reqHash instanceof HashObj) || !field) {
        return new ErrorObj("multipart_text expects (req, field)");
      }
      const rid = hashGetString(reqHash, "request_id");
      const form = forms.get(rid);
      return new StringObj(form?.fields[field] ?? "");
    });

    setBuiltin(env, "multipart_file", (...args) => {
      const err = expectArgs("multipart_file", 2, args);
      if (err) {
        return err;
      }
      const reqHash = args[0];
      const field = asString(args[1]);
      if (!(reqHash instanceof HashObj) || !field) {
        return new ErrorObj("multipart_file expects (req, field)");
      }
      const rid = hashGetString(reqHash, "request_id");
      const form = forms.get(rid);
      const file = form?.files[field];
      if (!file) {
        return NULL_OBJ;
      }
      const sum = crypto.createHash("sha256").update(file.data).digest("hex");
      return fromJs({
        filename: file.filename,
        size: file.data.length,
        data: file.data.toString("binary"),
        sha256: "sha256:" + sum,
      });
    });

    setBuiltin(env, "send_email", (...args) => {
      const err = expectArgs("send_email", 3, args);
      if (err) {
        return err;
      }
      // eslint-disable-next-line no-console
      console.log(
        `[email] to=${asString(args[0])} subject=${asString(args[1])}\n${asString(args[2])}`,
      );
      return TRUE_OBJ;
    });

    installDbBuiltins(env);
  }

  function installDbBuiltins(env: Environment): void {
    const stubErr = (name: string) =>
      new ErrorObj(`${name}: not available in memory demo mode`);

    setBuiltin(env, "db_mode", () => new StringObj(dbMode));
    setBuiltin(env, "db_ping", () => TRUE_OBJ);

    setBuiltin(env, "db_count_packages", () =>
      new IntegerObj(
        isPostgres(db)
          ? Number(db.hubStats().Packages ?? 0)
          : db.packages.length,
      ),
    );
    setBuiltin(env, "db_count_versions", () =>
      new IntegerObj(
        isPostgres(db)
          ? Number(db.hubStats().Versions ?? 0)
          : db.versions.length,
      ),
    );
    setBuiltin(env, "db_count_users", () =>
      new IntegerObj(
        isPostgres(db)
          ? Math.max(Number(db.hubStats().Users ?? 0), 1)
          : Math.max(db.users.length, 1),
      ),
    );
    setBuiltin(env, "db_sum_downloads", () =>
      new IntegerObj(
        isPostgres(db)
          ? Number(db.hubStats().Downloads ?? 0)
          : db.packages.reduce((a, p) => a + p.DownloadCount, 0),
      ),
    );
    setBuiltin(env, "db_top_tags", (...args) => {
      const limit = asInt(args[0]) ?? 16;
      return fromJs(db.topTags(limit));
    });
    setBuiltin(env, "db_hub_stats", () => fromJs(db.hubStats()));
    setBuiltin(env, "db_list_recent", (...args) => {
      const limit = asInt(args[0]) ?? 10;
      return fromJs(db.listRecent(limit));
    });
    setBuiltin(env, "db_list_popular", (...args) => {
      const limit = asInt(args[0]) ?? 10;
      return fromJs(db.listPopular(limit));
    });
    setBuiltin(env, "db_search", (...args) => {
      if (args.length < 1) {
        return new ErrorObj("db_search wants opts hash");
      }
      if (args[0] instanceof HashObj) {
        return fromJs(db.search(toJs(args[0]) as Record<string, unknown>));
      }
      const q = asString(args[0]) || "";
      return fromJs(db.search({ q, browse: true }).packages);
    });
    setBuiltin(env, "db_list_licenses", (...args) =>
      fromJs(db.listLicenses(asInt(args[0]) ?? 20)),
    );
    setBuiltin(env, "db_top_keywords", (...args) =>
      fromJs(db.topKeywords(asInt(args[0]) ?? 24)),
    );
    setBuiltin(env, "db_list_packages_page", (...args) => {
      const page = asInt(args[0]) ?? 1;
      const per = asInt(args[1]) ?? 25;
      const res = db.search({ browse: true, sort: "downloads", limit: per, offset: (page - 1) * per });
      return fromJs({
        packages: res.packages,
        page: { Page: page, PerPage: per, Total: res.total, HasNext: page * per < res.total, HasPrev: page > 1 },
      });
    });
    setBuiltin(env, "db_get_package", (...args) => {
      const name = asString(args[0]);
      if (!name) {
        return NULL_OBJ;
      }
      const pkg = db.getPackage(name);
      return pkg ? fromJs(pkg) : NULL_OBJ;
    });
    setBuiltin(env, "db_list_versions", (...args) => {
      const id = asInt(args[0]);
      if (id === null) {
        return fromJs([]);
      }
      return fromJs(db.listVersions(id));
    });
    setBuiltin(env, "db_get_package_version", (...args) => {
      const name = asString(args[0]);
      const ver = asString(args[1]);
      if (!name || !ver) {
        return NULL_OBJ;
      }
      const pv = db.getPackageVersion(name, ver);
      return pv ? fromJs(pv) : NULL_OBJ;
    });
    setBuiltin(env, "db_list_deps", () => fromJs([]));
    setBuiltin(env, "db_list_reverse_deps", () => fromJs([]));
    setBuiltin(env, "db_list_daily_downloads", (...args) => {
      const days = asInt(args[1]) ?? 30;
      const out = [];
      const now = Date.now();
      for (let i = days - 1; i >= 0; i--) {
        const d = new Date(now - i * 86400000).toISOString().slice(0, 10);
        out.push({ Date: d, Count: Math.max(0, 3 - (i % 5)) });
      }
      return fromJs(out);
    });
    setBuiltin(env, "db_increment_downloads", (...args) => {
      const id = asInt(args[0]);
      if (id === null) {
        return TRUE_OBJ;
      }
      if (isPostgres(db)) {
        db.incrementDownloads(id);
        return TRUE_OBJ;
      }
      const pkg = db.packages.find((p) => p.ID === id);
      if (pkg) {
        pkg.DownloadCount += 1;
      }
      return TRUE_OBJ;
    });
    setBuiltin(env, "db_get_user", (...args) => {
      const u = db.getUser(asString(args[0]) || "");
      return u ? fromJs(u) : NULL_OBJ;
    });
    setBuiltin(env, "db_get_user_by_login", (...args) => {
      const u = db.getUser(asString(args[0]) || "");
      return u ? fromJs(u) : NULL_OBJ;
    });
    setBuiltin(env, "db_get_user_by_email", (...args) => {
      const email = (asString(args[0]) || "").toLowerCase();
      if (isPostgres(db)) {
        const found = db.getUser(email);
        return found ? fromJs(found) : NULL_OBJ;
      }
      const u = db.users.find((x) => x.email.toLowerCase() === email);
      return u ? fromJs(u) : NULL_OBJ;
    });
    setBuiltin(env, "db_create_user", (...args) => {
      const username = asString(args[0]) || "";
      const email = asString(args[1]) || "";
      const hash = asString(args[2]) || "";
      if (!username || !email) {
        return new ErrorObj("db_create_user: username and email required");
      }
      try {
        if (db.getUser(username) || db.users.some((u) => u.email === email)) {
          return new ErrorObj("conflict: user already exists");
        }
        return fromJs(
          db.createUser(
            username,
            email,
            hash,
            asString(args[3]) || "",
            asString(args[4]) || "",
            asBool(args[5]),
          ),
        );
      } catch (e) {
        return new ErrorObj(e instanceof Error ? e.message : String(e));
      }
    });
    setBuiltin(env, "db_create_session", (...args) => {
      const id = asInt(args[0]);
      if (id === null) {
        return new ErrorObj("db_create_session wants user id");
      }
      return new StringObj(db.createSession(id));
    });
    setBuiltin(env, "db_delete_session", (...args) => {
      db.deleteSession(asString(args[0]) || "");
      return TRUE_OBJ;
    });
    setBuiltin(env, "db_user_stats", (...args) => {
      const userId = asInt(args[0]);
      if (isPostgres(db) && userId !== null) {
        return fromJs(db.userStats(userId));
      }
      return fromJs({
        PackageCount: db.packages.length,
        VersionCount: db.versions.length,
        TotalDownloads: db.packages.reduce((a, p) => a + p.DownloadCount, 0),
        APIKeyCount: 0,
        TrustedCount: 0,
        package_count: db.packages.length,
        version_count: db.versions.length,
        total_downloads: db.packages.reduce((a, p) => a + p.DownloadCount, 0),
      });
    });
    setBuiltin(env, "db_list_packages_by_owner", (...args) => {
      const userId = asInt(args[0]);
      if (isPostgres(db) && userId !== null) {
        return fromJs(db.listPackagesByOwner(userId));
      }
      return fromJs(db.listRecent(50));
    });
    setBuiltin(env, "db_list_user_activity", () => fromJs([]));
    setBuiltin(env, "db_list_user_orgs", (...args) => {
      const userId = asInt(args[0]);
      if (isPostgres(db) && userId !== null) {
        return fromJs(db.listUserOrgs(userId));
      }
      return fromJs([]);
    });
    setBuiltin(env, "db_list_api_keys", (...args) => {
      const userId = asInt(args[0]);
      if (isPostgres(db) && userId !== null) {
        return fromJs(db.listApiKeys(userId));
      }
      return fromJs([]);
    });
    setBuiltin(env, "db_list_trusted", (...args) => {
      const userId = asInt(args[0]);
      if (isPostgres(db) && userId !== null) {
        return fromJs(db.listTrusted(userId));
      }
      return fromJs([]);
    });
    setBuiltin(env, "db_list_audit_logs", () => fromJs([]));
    setBuiltin(env, "db_list_audit_logs_admin", () => fromJs([]));
    setBuiltin(env, "db_list_abuse_reports", () => fromJs(db.reports));
    setBuiltin(env, "db_list_package_owners", (...args) => {
      const pkgId = asInt(args[0]);
      if (isPostgres(db) && pkgId !== null) {
        return fromJs(db.listPackageOwners(pkgId));
      }
      return fromJs([]);
    });
    setBuiltin(env, "db_package_owner_role", (...args) => {
      const pkgId = asInt(args[0]);
      const userId = asInt(args[1]);
      if (isPostgres(db) && pkgId !== null && userId !== null) {
        return new StringObj(db.packageOwnerRole(pkgId, userId));
      }
      return new StringObj("");
    });
    setBuiltin(env, "db_user_can_publish", () => TRUE_OBJ);
    setBuiltin(env, "db_update_profile", (...args) => {
      const id = asInt(args[0]);
      if (id === null) {
        return new ErrorObj("user not found");
      }
      if (isPostgres(db)) {
        const updated = db.updateProfile(
          id,
          asString(args[1]) || "",
          asString(args[2]) || "",
          asBool(args[3]),
        );
        return updated ? fromJs(updated) : new ErrorObj("user not found");
      }
      const u = db.getUserById(id);
      if (!u) {
        return new ErrorObj("user not found");
      }
      u.bio = asString(args[1]) || "";
      u.Bio = u.bio;
      u.avatar_url = asString(args[2]) || "";
      u.AvatarURL = u.avatar_url;
      u.use_gravatar = asBool(args[3]);
      u.UseGravatar = u.use_gravatar;
      return fromJs(u);
    });
    setBuiltin(env, "db_unlink_github", () => TRUE_OBJ);
    setBuiltin(env, "db_create_api_key", (...args) => {
      const userId = asInt(args[0]);
      if (isPostgres(db) && userId !== null) {
        return fromJs(
          db.createApiKey(
            userId,
            asString(args[1]) || "api key",
            asString(args[2]) || undefined,
            asInt(args[3]) ?? undefined,
          ),
        );
      }
      return stubErr("db_create_api_key");
    });
    setBuiltin(env, "db_revoke_api_key", (...args) => {
      const userId = asInt(args[0]);
      const keyId = asInt(args[1]);
      if (isPostgres(db) && userId !== null && keyId !== null) {
        db.revokeApiKey(userId, keyId);
      }
      return TRUE_OBJ;
    });
    setBuiltin(env, "db_create_trusted", () => stubErr("db_create_trusted"));
    setBuiltin(env, "db_delete_trusted", () => TRUE_OBJ);
    setBuiltin(env, "db_match_trusted", () => NULL_OBJ);
    setBuiltin(env, "db_match_trusted_claims", () => NULL_OBJ);
    setBuiltin(env, "db_explain_trusted_mismatch", () => new StringObj("no match"));
    setBuiltin(env, "db_mint_publish_token", () =>
      fromJs({ token: "nxs_demo_" + crypto.randomBytes(8).toString("hex"), expires_in: 900 }),
    );
    setBuiltin(env, "db_publish", () => stubErr("db_publish"));
    setBuiltin(env, "db_yank_version", () => stubErr("db_yank_version"));
    setBuiltin(env, "db_unyank_version", () => stubErr("db_unyank_version"));
    setBuiltin(env, "db_deprecate_version", () => stubErr("db_deprecate_version"));
    setBuiltin(env, "db_unpublish_version", () => stubErr("db_unpublish_version"));
    setBuiltin(env, "db_audit_log", () => TRUE_OBJ);
    setBuiltin(env, "db_create_abuse_report", (...args) => {
      const data = args[0] instanceof HashObj ? (toJs(args[0]) as Record<string, unknown>) : {};
      if (isPostgres(db)) {
        return fromJs(db.createAbuseReport(data));
      }
      const row = { ID: db.reports.length + 1, ...data, Status: "open" };
      db.reports.push(row);
      return fromJs(row);
    });
    setBuiltin(env, "db_create_auth_token", (...args) => {
      const userId = asInt(args[0]);
      if (isPostgres(db) && userId !== null) {
        return new StringObj(
          db.createAuthToken(
            userId,
            asString(args[1]) || "verify",
            asInt(args[2]) ?? 3600,
          ),
        );
      }
      return new StringObj("tok_" + crypto.randomBytes(16).toString("hex"));
    });
    setBuiltin(env, "db_consume_auth_token", (...args) => {
      if (isPostgres(db)) {
        const u = db.consumeAuthToken(
          asString(args[0]) || "",
          asString(args[1]) || "verify",
        );
        return u ? fromJs(u) : NULL_OBJ;
      }
      return NULL_OBJ;
    });
    setBuiltin(env, "db_peek_auth_token", (...args) => {
      if (isPostgres(db)) {
        const u = db.peekAuthToken(
          asString(args[0]) || "",
          asString(args[1]) || "verify",
        );
        return u ? fromJs(u) : NULL_OBJ;
      }
      return NULL_OBJ;
    });
    setBuiltin(env, "db_mark_email_verified", (...args) => {
      const userId = asInt(args[0]);
      if (isPostgres(db) && userId !== null) {
        db.markEmailVerified(userId);
      }
      return TRUE_OBJ;
    });
    setBuiltin(env, "db_set_password", (...args) => {
      const userId = asInt(args[0]);
      if (isPostgres(db) && userId !== null) {
        db.setPassword(userId, asString(args[1]) || "");
      }
      return TRUE_OBJ;
    });
    setBuiltin(env, "db_totp_begin", () => stubErr("db_totp_begin"));
    setBuiltin(env, "db_totp_confirm", () => stubErr("db_totp_confirm"));
    setBuiltin(env, "db_totp_disable", () => stubErr("db_totp_disable"));
    setBuiltin(env, "db_totp_challenge_create", () => stubErr("db_totp_challenge_create"));
    setBuiltin(env, "db_totp_challenge_consume", () => stubErr("db_totp_challenge_consume"));
    setBuiltin(env, "db_mark_trusted_verified", () => TRUE_OBJ);
    setBuiltin(env, "db_record_trusted_failure", () => TRUE_OBJ);
    setBuiltin(env, "db_set_version_provenance", () => TRUE_OBJ);
    setBuiltin(env, "db_get_org", (...args) => {
      if (isPostgres(db)) {
        const org = db.getOrg(asString(args[0]) || "");
        return org ? fromJs(org) : NULL_OBJ;
      }
      return NULL_OBJ;
    });
    setBuiltin(env, "db_create_org", (...args) => {
      const userId = asInt(args[3]);
      if (isPostgres(db) && userId !== null) {
        const org = db.createOrg(
          asString(args[0]) || "",
          asString(args[1]) || "",
          asString(args[2]) || "",
          userId,
        );
        return org ? fromJs(org) : stubErr("db_create_org");
      }
      return stubErr("db_create_org");
    });
    setBuiltin(env, "db_list_org_members", () => fromJs([]));
    setBuiltin(env, "db_list_org_packages", () => fromJs([]));
    setBuiltin(env, "db_list_org_activity", () => fromJs([]));
    setBuiltin(env, "db_list_teams", () => fromJs([]));
    setBuiltin(env, "db_org_member_role", () => new StringObj(""));
    setBuiltin(env, "db_add_org_member", () => TRUE_OBJ);
    setBuiltin(env, "db_remove_org_member", () => TRUE_OBJ);
    setBuiltin(env, "db_create_team", () => stubErr("db_create_team"));
    setBuiltin(env, "db_add_team_member", () => TRUE_OBJ);
    setBuiltin(env, "db_list_team_members", () => fromJs([]));
    setBuiltin(env, "db_invite_package_owner", () => stubErr("db_invite_package_owner"));
    setBuiltin(env, "db_add_org_package_owner", () => TRUE_OBJ);
    setBuiltin(env, "db_remove_package_owner", () => TRUE_OBJ);
    setBuiltin(env, "db_accept_owner_invite", () => TRUE_OBJ);
    setBuiltin(env, "db_create_ownership_transfer", () => stubErr("db_create_ownership_transfer"));
    setBuiltin(env, "db_accept_ownership_transfer", () => TRUE_OBJ);
  }

  async function readBody(req: http.IncomingMessage, limit: number): Promise<Buffer> {
    return new Promise((resolve, reject) => {
      const chunks: Buffer[] = [];
      let size = 0;
      req.on("data", (c: Buffer) => {
        size += c.length;
        if (size > limit) {
          reject(new Error("body too large"));
          req.destroy();
          return;
        }
        chunks.push(c);
      });
      req.on("end", () => resolve(Buffer.concat(chunks)));
      req.on("error", reject);
    });
  }

  async function handleRequest(
    req: http.IncomingMessage,
    res: http.ServerResponse,
  ): Promise<void> {
    const hostHeader = req.headers.host || "localhost";
    const url = new URL(req.url || "/", `http://${hostHeader}`);
    const pathname = decodeURIComponent(url.pathname);

    // Static files
    if (pathname === "/static/js/nxd.js") {
      res.statusCode = 200;
      res.setHeader("Content-Type", "application/javascript; charset=utf-8");
      res.setHeader("Cache-Control", "public, max-age=3600");
      res.end(nxdKitScript());
      return;
    }
    if (pathname.startsWith("/static/") && config.webDir) {
      const rel = pathname.slice("/static/".length);
      const filePath = path.normalize(path.join(config.webDir, "static", rel));
      const root = path.normalize(path.join(config.webDir, "static"));
      if (!filePath.startsWith(root) || !fs.existsSync(filePath) || !fs.statSync(filePath).isFile()) {
        res.statusCode = 404;
        res.end("not found");
        return;
      }
      const ext = path.extname(filePath).toLowerCase();
      const types: Record<string, string> = {
        ".css": "text/css; charset=utf-8",
        ".js": "application/javascript; charset=utf-8",
        ".svg": "image/svg+xml",
        ".png": "image/png",
        ".jpg": "image/jpeg",
        ".jpeg": "image/jpeg",
        ".ico": "image/x-icon",
        ".woff2": "font/woff2",
      };
      res.statusCode = 200;
      res.setHeader("Content-Type", types[ext] || "application/octet-stream");
      fs.createReadStream(filePath).pipe(res);
      return;
    }

    if (pathname === "/metrics") {
      if (isPostgres(db)) {
        db.refreshCaches();
      }
      const pkgCount = db.packages.length;
      res.statusCode = 200;
      res.setHeader("Content-Type", "text/plain; version=0.0.4");
      res.end(
        `# TYPE nex_up gauge\nnex_up 1\n# TYPE nex_packages gauge\nnex_packages ${pkgCount}\n# TYPE nex_db_mode gauge\nnex_db_postgres ${isPostgres(db) ? 1 : 0}\n`,
      );
      return;
    }

    const method = (req.method || "GET").toUpperCase();
    const match = router.match(method, pathname);
    const { user, via } = resolveUser(req);
    const requestId = crypto.randomBytes(8).toString("hex");

    if (match?.requireAuth && !user) {
      if (pathname.startsWith("/api/")) {
        res.statusCode = 401;
        res.setHeader("Content-Type", "application/json");
        res.end(JSON.stringify({ error: "authentication required" }));
        return;
      }
      res.statusCode = 303;
      res.setHeader("Location", `/login?next=${encodeURIComponent(pathname)}`);
      res.end();
      return;
    }

    const headers: Record<string, string> = {};
    for (const [k, v] of Object.entries(req.headers)) {
      if (typeof v === "string") {
        headers[k.toLowerCase()] = v;
      } else if (Array.isArray(v) && v[0]) {
        headers[k.toLowerCase()] = v[0];
      }
    }

    const incoming: IncomingRequest = {
      method,
      path: pathname,
      query: parseQuery(url),
      headers,
      params: match?.params ?? {},
      cookies: parseCookies(req.headers.cookie),
      form: {},
      body: "",
      requestId,
      user,
      authVia: via,
    };

    const ct = (req.headers["content-type"] || "").toLowerCase();
    if (method === "POST" || method === "PUT" || method === "PATCH") {
      try {
        const raw = await readBody(req, config.maxUploadBytes);
        if (ct.includes("multipart/form-data")) {
          const m = /boundary=(?:"([^"]+)"|([^;]+))/i.exec(ct);
          const boundary = (m?.[1] || m?.[2] || "").trim();
          if (boundary) {
            const parsed = parseMultipart(raw, boundary);
            forms.set(requestId, parsed);
            incoming.form = parsed.fields;
          }
        } else if (ct.includes("application/x-www-form-urlencoded")) {
          incoming.form = parseFormUrlEncoded(raw.toString("utf8"));
        } else {
          incoming.body = raw.toString("utf8");
        }
      } catch (e) {
        res.statusCode = 413;
        res.end(e instanceof Error ? e.message : "body error");
        return;
      }
    }

    const handler =
      match?.handler ??
      router.notFoundHandler ??
      (() => {
        const out = new HashObj();
        out.setString("status", new IntegerObj(404));
        out.setString("json", fromJs({ error: "not found" }));
        return out;
      });

    try {
      const result = handler(incoming);
      const value = result instanceof Promise ? await result : result;
      const nex =
        value && typeof value === "object" && "type" in (value as object)
          ? (value as NexusObject)
          : fromJs(value);
      if (nex instanceof ErrorObj) {
        // eslint-disable-next-line no-console
        console.error("nex handler error", pathname, nex.message);
        const accept = req.headers.accept || "";
        if (accept.includes("text/html")) {
          writeResponse(
            res,
            req,
            (() => {
              const out = new HashObj();
              out.setString("status", new IntegerObj(500));
              out.setString("html", new StringObj("error.html"));
              out.setString(
                "data",
                fromJs({ Title: "Error", Status: 500, Message: "Internal error" }),
              );
              return out;
            })(),
            user,
          );
          return;
        }
        res.statusCode = 500;
        res.setHeader("Content-Type", "application/json");
        res.end(JSON.stringify({ error: "internal error", details: nex.message }));
        return;
      }
      writeResponse(res, req, nex, user);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error("handler exception", err);
      res.statusCode = 500;
      res.end("internal error");
    } finally {
      forms.delete(requestId);
    }
  }

  function listenAndServe(): Promise<void> {
    const { host: listenHost, port } = parseListenAddr(config.listenAddr);
    return new Promise((resolve, reject) => {
      server = http.createServer((req, res) => {
        void handleRequest(req, res);
      });
      server.on("error", reject);
      server.listen(port, listenHost, () => {
        // eslint-disable-next-line no-console
        console.log(
          JSON.stringify({
            msg: "nex listening",
            addr: config.listenAddr,
            host: listenHost,
            port,
            storage: config.storageDir,
            base_url: config.baseUrl,
            app: config.appDir,
            web: config.webDir,
            routes: router.routeCount,
            db: dbMode,
          }),
        );
        // Keep promise pending until close — CLI awaits this
        server!.on("close", () => resolve());
      });
    });
  }

  return host;
}
