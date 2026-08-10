/**
 * Go html/template subset used by Nexus Registry templates.
 *
 * Supports: define/template, if/else/end, range/end, field paths,
 * eq/ne, and helpers formatNumber / formatDate / formatBytes / initial.
 */

export class SafeHtml {
  constructor(readonly html: string) {}
}

type Data = unknown;

interface TemplateSet {
  [name: string]: string;
}

type Token =
  | { kind: "text"; value: string }
  | { kind: "action"; value: string };

function tokenize(src: string): Token[] {
  const out: Token[] = [];
  let i = 0;
  while (i < src.length) {
    const start = src.indexOf("{{", i);
    if (start < 0) {
      out.push({ kind: "text", value: src.slice(i) });
      break;
    }
    if (start > i) {
      out.push({ kind: "text", value: src.slice(i, start) });
    }
    const end = src.indexOf("}}", start);
    if (end < 0) {
      out.push({ kind: "text", value: src.slice(start) });
      break;
    }
    out.push({ kind: "action", value: src.slice(start + 2, end).trim() });
    i = end + 2;
  }
  return out;
}

function extractDefines(src: string): TemplateSet {
  // Depth-aware: a naive non-greedy {{define}}...{{end}} regex stops at the
  // first nested {{if}}/{{range}}/{{with}} {{end}}, truncating "base" mid-<title>.
  const set: TemplateSet = {};
  const tokens = tokenize(src);
  let i = 0;
  while (i < tokens.length) {
    const tok = tokens[i++]!;
    if (tok.kind !== "action") {
      continue;
    }
    const def = tok.value.match(/^define\s+"([^"]+)"\s*$/);
    if (!def) {
      continue;
    }
    const name = def[1]!;
    let depth = 1;
    let body = "";
    while (i < tokens.length) {
      const t = tokens[i++]!;
      if (t.kind === "text") {
        body += t.value;
        continue;
      }
      const a = t.value;
      const head = a.split(/\s+/)[0] ?? "";
      if (head === "if" || head === "range" || head === "with") {
        depth++;
        body += `{{${a}}}`;
        continue;
      }
      if (a === "end") {
        depth--;
        if (depth === 0) {
          break;
        }
        body += `{{${a}}}`;
        continue;
      }
      body += `{{${a}}}`;
    }
    set[name] = body;
  }
  return set;
}

function htmlEscape(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function isTruthy(v: Data): boolean {
  if (v === null || v === undefined || v === false || v === 0 || v === "") {
    return false;
  }
  if (Array.isArray(v)) {
    return v.length > 0;
  }
  return true;
}

function lookup(data: Data, path: string): Data {
  if (path === "." || path === "$") {
    return data;
  }
  let cur: Data = data;
  const parts = path.replace(/^\./, "").split(".");
  for (const part of parts) {
    if (cur === null || cur === undefined) {
      return undefined;
    }
    if (typeof cur !== "object") {
      return undefined;
    }
    cur = (cur as Record<string, unknown>)[part];
  }
  return cur;
}

function formatBytes(n: number): string {
  if (n < 1024) {
    return `${n} B`;
  }
  const units = ["KiB", "MiB", "GiB"];
  let val = n;
  for (const unit of units) {
    val /= 1024;
    if (val < 1024) {
      return `${val.toFixed(1)} ${unit}`;
    }
  }
  return `${val.toFixed(1)} GiB`;
}

function formatDate(v: Data): string {
  if (v === null || v === undefined) {
    return "";
  }
  if (typeof v === "string") {
    const s = v.trim();
    if (!s) {
      return "";
    }
    const d = Date.parse(s);
    if (!Number.isNaN(d)) {
      return new Date(d).toISOString().slice(0, 10);
    }
    return s.length >= 10 ? s.slice(0, 10) : s;
  }
  return String(v);
}

function formatNumber(v: Data): string {
  const n =
    typeof v === "number"
      ? v
      : typeof v === "string"
        ? Number.parseInt(v, 10)
        : Number.NaN;
  if (Number.isNaN(n)) {
    return String(v ?? "");
  }
  return Math.trunc(n).toLocaleString("en-US");
}

function initial(s: Data): string {
  const t = String(s ?? "").trim();
  return t ? t[0]!.toUpperCase() : "?";
}

function parseArgs(src: string): string[] {
  const out: string[] = [];
  let i = 0;
  while (i < src.length) {
    while (i < src.length && /\s/.test(src[i]!)) {
      i++;
    }
    if (i >= src.length) {
      break;
    }
    if (src[i] === '"') {
      i++;
      let s = "";
      while (i < src.length && src[i] !== '"') {
        s += src[i++];
      }
      i++;
      out.push(JSON.stringify(s));
    } else {
      let s = "";
      while (i < src.length && !/\s/.test(src[i]!)) {
        s += src[i++];
      }
      out.push(s);
    }
  }
  return out;
}

function evalPipeline(expr: string, data: Data, root: Data): Data {
  const trimmed = expr.trim();
  if (!trimmed) {
    return data;
  }
  if (trimmed.startsWith('"') && trimmed.endsWith('"')) {
    return JSON.parse(trimmed) as string;
  }
  if (/^-?\d+$/.test(trimmed)) {
    return Number.parseInt(trimmed, 10);
  }
  if (trimmed === "true") {
    return true;
  }
  if (trimmed === "false") {
    return false;
  }

  const parts = parseArgs(trimmed);
  if (parts.length === 0) {
    return undefined;
  }

  const head = parts[0]!;
  if (head.startsWith(".")) {
    return lookup(data, head);
  }

  const args = parts.slice(1).map((a) => {
    if (a.startsWith('"')) {
      return JSON.parse(a) as string;
    }
    if (/^-?\d+$/.test(a)) {
      return Number.parseInt(a, 10);
    }
    if (a.startsWith(".")) {
      return lookup(data, a);
    }
    return lookup(data, `.${a}`);
  });

  switch (head) {
    case "eq":
      return String(args[0]) === String(args[1]);
    case "ne":
      return String(args[0]) !== String(args[1]);
    case "formatNumber":
      return formatNumber(args[0]);
    case "formatDate":
      return formatDate(args[0]);
    case "formatBytes":
      return formatBytes(Number(args[0] ?? 0));
    case "initial":
      return initial(args[0]);
    default:
      // Unknown function: try as field on root
      return lookup(root, `.${head}`);
  }
}

function renderValue(v: Data, escape: boolean): string {
  if (v instanceof SafeHtml) {
    return v.html;
  }
  if (v === null || v === undefined) {
    return "";
  }
  const s = String(v);
  return escape ? htmlEscape(s) : s;
}

class Renderer {
  constructor(private readonly templates: TemplateSet) {}

  render(name: string, data: Data): string {
    const src = this.templates[name];
    if (src === undefined) {
      throw new Error(`template not found: ${name}`);
    }
    return this.renderSrc(src, data, data);
  }

  private renderSrc(src: string, data: Data, root: Data): string {
    const tokens = tokenize(src);
    let i = 0;
    let out = "";

    const next = () => tokens[i++];

    while (i < tokens.length) {
      const tok = next()!;
      if (tok.kind === "text") {
        out += tok.value;
        continue;
      }
      const action = tok.value;
      if (action.startsWith("/*") || action.startsWith("- ")) {
        continue;
      }
      if (action === "end") {
        continue;
      }
      if (action.startsWith("define ")) {
        continue;
      }

      if (action.startsWith("if ")) {
        const condExpr = action.slice(3).trim();
        const { body, elseBody } = this.readIfBodies(tokens, () => i, (n) => {
          i = n;
        });
        const cond = evalPipeline(condExpr, data, root);
        out += this.renderSrc(isTruthy(cond) ? body : elseBody, data, root);
        continue;
      }

      if (action.startsWith("range ")) {
        const rangeExpr = action.slice(5).trim();
        const body = this.readUntilEnd(tokens, () => i, (n) => {
          i = n;
        });
        const coll = evalPipeline(rangeExpr, data, root);
        if (Array.isArray(coll)) {
          for (const item of coll) {
            out += this.renderSrc(body, item, root);
          }
        } else if (coll && typeof coll === "object") {
          for (const [k, v] of Object.entries(coll as Record<string, unknown>)) {
            out += this.renderSrc(body, { Key: k, Value: v }, root);
          }
        }
        continue;
      }

      if (action.startsWith("with ")) {
        const withExpr = action.slice(5).trim();
        const { body, elseBody } = this.readIfBodies(tokens, () => i, (n) => {
          i = n;
        });
        const nextData = evalPipeline(withExpr, data, root);
        if (isTruthy(nextData)) {
          out += this.renderSrc(body, nextData, root);
        } else {
          out += this.renderSrc(elseBody, data, root);
        }
        continue;
      }

      if (action.startsWith("template ")) {
        const m = action.match(/^template\s+"([^"]+)"(?:\s+(.+))?$/);
        if (m) {
          const tname = m[1]!;
          const pipe = (m[2] ?? ".").trim();
          const ctx = evalPipeline(pipe, data, root);
          out += this.render(tname, ctx);
        }
        continue;
      }

      const val = evalPipeline(action, data, root);
      out += renderValue(val, true);
    }

    return out;
  }

  private readUntilEnd(
    tokens: Token[],
    getI: () => number,
    setI: (n: number) => void,
  ): string {
    let depth = 1;
    let buf = "";
    let i = getI();
    while (i < tokens.length) {
      const tok = tokens[i++]!;
      if (tok.kind === "text") {
        buf += tok.value;
        continue;
      }
      const a = tok.value;
      const head = a.split(/\s+/)[0] ?? "";
      if (head === "if" || head === "range" || head === "with") {
        depth++;
        buf += `{{${a}}}`;
        continue;
      }
      if (a === "end") {
        depth--;
        if (depth === 0) {
          break;
        }
        buf += `{{${a}}}`;
        continue;
      }
      buf += `{{${a}}}`;
    }
    setI(i);
    return buf;
  }

  private readIfBodies(
    tokens: Token[],
    getI: () => number,
    setI: (n: number) => void,
  ): { body: string; elseBody: string } {
    let depth = 1;
    let body = "";
    let elseBody = "";
    let inElse = false;
    let i = getI();
    while (i < tokens.length) {
      const tok = tokens[i++]!;
      if (tok.kind === "text") {
        if (inElse) {
          elseBody += tok.value;
        } else {
          body += tok.value;
        }
        continue;
      }
      const a = tok.value;
      const head = a.split(/\s+/)[0] ?? "";
      if (head === "if" || head === "range" || head === "with") {
        depth++;
        if (inElse) {
          elseBody += `{{${a}}}`;
        } else {
          body += `{{${a}}}`;
        }
        continue;
      }
      if (a === "else" && depth === 1) {
        inElse = true;
        continue;
      }
      if (a === "end") {
        depth--;
        if (depth === 0) {
          break;
        }
        if (inElse) {
          elseBody += `{{${a}}}`;
        } else {
          body += `{{${a}}}`;
        }
        continue;
      }
      if (inElse) {
        elseBody += `{{${a}}}`;
      } else {
        body += `{{${a}}}`;
      }
    }
    setI(i);
    return { body, elseBody };
  }
}

export interface TemplateEngine {
  render(page: string, data: Record<string, unknown>): string;
}

export function loadGoTemplates(sources: {
  base: string;
  pages: Record<string, string>;
  partials: Record<string, string>;
}): TemplateEngine {
  const cache = new Map<string, Renderer>();

  const buildForPage = (pageSrc: string): Renderer => {
    const combined =
      sources.base +
      "\n" +
      Object.values(sources.partials).join("\n") +
      "\n" +
      pageSrc;
    const defines = extractDefines(combined);
    return new Renderer(defines);
  };

  return {
    render(page: string, data: Record<string, unknown>): string {
      let r = cache.get(page);
      if (!r) {
        const pageSrc = sources.pages[page];
        if (pageSrc === undefined) {
          throw new Error(`template page not found: ${page}`);
        }
        r = buildForPage(pageSrc);
        cache.set(page, r);
      }
      return r.render("base", data);
    },
  };
}

export function normalizeTemplateData(
  data: Record<string, unknown>,
): Record<string, unknown> {
  const out: Record<string, unknown> = { ...data };
  const aliases: Record<string, string> = {
    title: "Title",
    description: "Description",
    active_nav: "ActiveNav",
    query: "Query",
    category: "Category",
    result_count: "ResultCount",
    package_count: "PackageCount",
    version_count: "VersionCount",
    packages: "Packages",
    featured: "Featured",
    recent: "Recent",
    package: "Package",
    versions: "Versions",
    selected: "Selected",
    dependencies: "Dependencies",
    download_url: "DownloadURL",
    install_command: "InstallCommand",
    filename: "Filename",
    readme_html: "ReadmeHTML",
    doc_id: "DocID",
    doc_title: "DocTitle",
    doc_lead: "DocLead",
    doc_html: "DocHTML",
    status: "Status",
    message: "Message",
    is_search: "IsSearch",
    is_docs: "IsDocs",
    doc_section: "DocSection",
    user: "User",
    profile_user: "ProfileUser",
    stats: "Stats",
    download_count: "DownloadCount",
    user_count: "UserCount",
    tags: "Tags",
    prev_page: "PrevPage",
    next_page: "NextPage",
    categories: "Categories",
    keywords: "Keywords",
    docs_url: "DocsURL",
    docs_html: "DocsHTML",
    has_docs: "HasDocs",
    package_url: "PackageURL",
    is_version_docs: "IsVersionDocs",
    is_legal: "IsLegal",
    legal_id: "LegalID",
    legal_title: "LegalTitle",
    legal_lead: "LegalLead",
    legal_html: "LegalHTML",
    show_dmca_form: "ShowDmcaForm",
    dmca_submitted: "DmcaSubmitted",
    dmca_error: "DmcaError",
    form_action: "FormAction",
    current_user: "CurrentUser",
  };
  for (const [from, to] of Object.entries(aliases)) {
    if (out[to] === undefined && out[from] !== undefined) {
      out[to] = out[from];
    }
  }
  for (const key of ["ReadmeHTML", "DocHTML", "LegalHTML", "DocsHTML"]) {
    const v = out[key];
    if (typeof v === "string") {
      out[key] = new SafeHtml(v);
    }
  }
  return out;
}
