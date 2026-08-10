import * as crypto from "crypto";
import * as fs from "fs";
import * as fsp from "fs/promises";
import * as os from "os";
import * as path from "path";
import { pipeline } from "stream/promises";
import axios, { AxiosError } from "axios";
import FormData from "form-data";
import { parse as parseToml, stringify as stringifyToml } from "smol-toml";
import * as tar from "tar";

export const MANIFEST_FILENAME = "nexus.toml";
export const MODULES_DIRNAME = ".modules";
export const DEFAULT_REGISTRY_URL = "http://localhost:8080";

export interface NexusManifest {
  name: string;
  version: string;
  author: string;
  description?: string;
  dependencies?: Record<string, string>;
}

export interface PackageInfo {
  name: string;
  version: string;
  author: string;
  checksum: string;
  filename?: string;
  url?: string;
}

export interface PublishResult {
  package: PackageInfo;
  message?: string;
}

export interface RegistryClientOptions {
  registryUrl?: string;
  timeoutMs?: number;
}

const NAME_RE = /^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/;
const VERSION_RE = /^\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?$/;

/**
 * Registry & package-management client for Nexus workspaces.
 */
export class RegistryClient {
  private readonly registryUrl: string;
  private readonly timeoutMs: number;

  constructor(options: RegistryClientOptions = {}) {
    this.registryUrl = (options.registryUrl ?? DEFAULT_REGISTRY_URL).replace(
      /\/+$/,
      "",
    );
    this.timeoutMs = options.timeoutMs ?? 60_000;
  }

  get baseUrl(): string {
    return this.registryUrl;
  }

  /** Read and validate `nexus.toml` from a workspace directory. */
  async readManifest(workspaceRoot: string): Promise<NexusManifest> {
    const manifestPath = path.join(workspaceRoot, MANIFEST_FILENAME);
    let raw: string;
    try {
      raw = await fsp.readFile(manifestPath, "utf8");
    } catch (err) {
      throw new Error(
        `unable to read ${MANIFEST_FILENAME}: ${errorMessage(err)}`,
      );
    }

    let parsed: unknown;
    try {
      parsed = parseToml(raw);
    } catch (err) {
      throw new Error(`invalid ${MANIFEST_FILENAME}: ${errorMessage(err)}`);
    }

    const manifest = normalizeManifest(parsed);
    validateManifest(manifest);
    return manifest;
  }

  /** Write a manifest to the workspace root. */
  async writeManifest(
    workspaceRoot: string,
    manifest: NexusManifest,
  ): Promise<void> {
    validateManifest(manifest);
    const payload: Record<string, unknown> = {
      name: manifest.name,
      version: manifest.version,
      author: manifest.author,
    };
    if (manifest.description) {
      payload.description = manifest.description;
    }
    payload.dependencies = manifest.dependencies ?? {};

    const content = stringifyToml(payload);
    await fsp.writeFile(
      path.join(workspaceRoot, MANIFEST_FILENAME),
      content.endsWith("\n") ? content : `${content}\n`,
      "utf8",
    );
  }

  /**
   * Compress a workspace directory into a `.tar.gz` archive.
   * Skips `.git`, `.modules`, and the destination archive itself.
   */
  async createArchive(
    workspaceRoot: string,
    destArchivePath: string,
  ): Promise<void> {
    const absDest = path.resolve(destArchivePath);
    await fsp.mkdir(path.dirname(absDest), { recursive: true });

    const skipTopLevel = new Set([".git", ".modules", ".nex", "node_modules"]);

    await tar.c(
      {
        gzip: true,
        file: absDest,
        cwd: workspaceRoot,
        filter: (p) => {
          const normalized = p.replace(/\\/g, "/").replace(/^\.\//, "");
          if (normalized === "." || normalized === "") {
            return true;
          }
          const top = normalized.split("/")[0] ?? "";
          if (skipTopLevel.has(top)) {
            return false;
          }
          if (path.resolve(workspaceRoot, normalized) === absDest) {
            return false;
          }
          return true;
        },
      },
      ["."],
    );
  }

  /** Compute the hex-encoded SHA-256 checksum of a file. */
  async sha256File(filePath: string): Promise<string> {
    const hash = crypto.createHash("sha256");
    const stream = fs.createReadStream(filePath);
    await pipeline(stream, hash);
    return hash.digest("hex");
  }

  /** Query package metadata from the registry. */
  async getPackage(name: string, version?: string): Promise<PackageInfo> {
    if (!name) {
      throw new Error("package name is required");
    }

    const url = new URL(
      `${this.registryUrl}/api/v1/packages/${encodeURIComponent(name)}`,
    );
    if (version) {
      url.searchParams.set("version", version);
    }

    try {
      const response = await axios.get<PackageInfo>(url.toString(), {
        timeout: this.timeoutMs,
        headers: { Accept: "application/json" },
        validateStatus: () => true,
      });

      if (response.status === 404) {
        throw new Error(`package "${name}" not found in registry`);
      }
      if (response.status < 200 || response.status >= 300) {
        throw new Error(
          `registry returned ${response.status}: ${stringifyBody(response.data)}`,
        );
      }

      const info = response.data;
      if (!info || typeof info !== "object") {
        throw new Error("registry returned an invalid package payload");
      }
      return {
        name: info.name || name,
        version: info.version,
        author: info.author ?? "",
        checksum: info.checksum,
        filename: info.filename,
        url: info.url,
      };
    } catch (err) {
      if (err instanceof Error && err.message.startsWith("package ")) {
        throw err;
      }
      throw new Error(`query registry failed: ${errorMessage(err)}`);
    }
  }

  /** Download a package archive to `destPath`. */
  async downloadPackage(
    name: string,
    version: string | undefined,
    destPath: string,
  ): Promise<void> {
    const url = new URL(
      `${this.registryUrl}/api/v1/packages/${encodeURIComponent(name)}/download`,
    );
    if (version) {
      url.searchParams.set("version", version);
    }

    await fsp.mkdir(path.dirname(destPath), { recursive: true });

    try {
      const response = await axios.get(url.toString(), {
        timeout: this.timeoutMs,
        responseType: "stream",
        validateStatus: () => true,
      });

      if (response.status === 404) {
        throw new Error(`package "${name}" not found in registry`);
      }
      if (response.status < 200 || response.status >= 300) {
        throw new Error(`registry returned ${response.status} during download`);
      }

      await pipeline(response.data, fs.createWriteStream(destPath));
    } catch (err) {
      if (err instanceof Error && err.message.startsWith("package ")) {
        throw err;
      }
      throw new Error(`download failed: ${errorMessage(err)}`);
    }
  }

  /**
   * Bundle the workspace, checksum it, and multipart-upload to the registry.
   */
  async publish(workspaceRoot: string): Promise<PublishResult> {
    const manifest = await this.readManifest(workspaceRoot);
    const tmpDir = await fsp.mkdtemp(path.join(os.tmpdir(), "nexus-publish-"));
    const archiveName = `${manifest.name}-${manifest.version}.tar.gz`;
    const archivePath = path.join(tmpDir, archiveName);

    try {
      await this.createArchive(workspaceRoot, archivePath);
      const checksum = await this.sha256File(archivePath);

      const form = new FormData();
      form.append("name", manifest.name);
      form.append("version", manifest.version);
      form.append("author", manifest.author);
      form.append("checksum", checksum);
      form.append("file", fs.createReadStream(archivePath), {
        filename: archiveName,
        contentType: "application/gzip",
      });

      const response = await axios.post(
        `${this.registryUrl}/api/v1/packages`,
        form,
        {
          timeout: this.timeoutMs,
          headers: {
            ...form.getHeaders(),
            Accept: "application/json",
          },
          maxBodyLength: Infinity,
          maxContentLength: Infinity,
          validateStatus: () => true,
        },
      );

      if (response.status < 200 || response.status >= 300) {
        throw new Error(
          `publish failed (${response.status}): ${stringifyBody(response.data)}`,
        );
      }

      const body = response.data as Partial<PublishResult> | string;
      if (typeof body === "string") {
        return {
          package: {
            name: manifest.name,
            version: manifest.version,
            author: manifest.author,
            checksum,
            filename: archiveName,
          },
          message: body.trim(),
        };
      }

      return {
        package: {
          name: body.package?.name ?? manifest.name,
          version: body.package?.version ?? manifest.version,
          author: body.package?.author ?? manifest.author,
          checksum: body.package?.checksum ?? checksum,
          filename: body.package?.filename ?? archiveName,
        },
        message: body.message,
      };
    } finally {
      await fsp.rm(tmpDir, { recursive: true, force: true });
    }
  }

  /**
   * Download a package, verify its checksum, and extract into `.modules/<name>`.
   */
  async install(
    workspaceRoot: string,
    packageSpec: string,
  ): Promise<{ name: string; version: string; destination: string }> {
    const { name, version: requestedVersion } = parsePackageSpec(packageSpec);
    if (!name) {
      throw new Error("invalid package name");
    }

    const info = await this.getPackage(name, requestedVersion);
    const version = requestedVersion || info.version;
    if (!info.checksum) {
      throw new Error(
        `registry response for ${name}@${version} is missing checksum`,
      );
    }

    const tmpDir = await fsp.mkdtemp(path.join(os.tmpdir(), "nexus-install-"));
    const archiveName =
      info.filename || `${name}-${version}.tar.gz`;
    const archivePath = path.join(tmpDir, path.basename(archiveName));

    try {
      await this.downloadPackage(name, version, archivePath);
      const actual = await this.sha256File(archivePath);
      if (actual.toLowerCase() !== info.checksum.toLowerCase()) {
        throw new Error(
          `checksum mismatch for ${name}@${version}: expected ${info.checksum}, got ${actual}`,
        );
      }

      const destination = path.join(workspaceRoot, MODULES_DIRNAME, name);
      await fsp.rm(destination, { recursive: true, force: true });
      await fsp.mkdir(destination, { recursive: true });
      await tar.x({
        file: archivePath,
        cwd: destination,
        strict: true,
      });

      const manifestPath = path.join(workspaceRoot, MANIFEST_FILENAME);
      try {
        await fsp.access(manifestPath);
        const manifest = await this.readManifest(workspaceRoot);
        manifest.dependencies = {
          ...(manifest.dependencies ?? {}),
          [name]: version,
        };
        await this.writeManifest(workspaceRoot, manifest);
      } catch {
        // No local manifest — install still succeeds into .modules/
      }

      return { name, version, destination };
    } finally {
      await fsp.rm(tmpDir, { recursive: true, force: true });
    }
  }
}

export function parsePackageSpec(spec: string): {
  name: string;
  version?: string;
} {
  const trimmed = spec.trim();
  if (!trimmed) {
    return { name: "" };
  }
  const at = trimmed.lastIndexOf("@");
  if (at > 0) {
    return { name: trimmed.slice(0, at), version: trimmed.slice(at + 1) };
  }
  return { name: trimmed };
}

function normalizeManifest(value: unknown): NexusManifest {
  if (!value || typeof value !== "object") {
    throw new Error("manifest root must be a table");
  }
  const obj = value as Record<string, unknown>;
  const depsRaw = obj.dependencies;
  let dependencies: Record<string, string> | undefined;
  if (depsRaw && typeof depsRaw === "object") {
    dependencies = {};
    for (const [k, v] of Object.entries(depsRaw as Record<string, unknown>)) {
      dependencies[k] = String(v);
    }
  }

  return {
    name: String(obj.name ?? ""),
    version: String(obj.version ?? ""),
    author: String(obj.author ?? ""),
    description:
      obj.description !== undefined ? String(obj.description) : undefined,
    dependencies,
  };
}

function validateManifest(manifest: NexusManifest): void {
  if (!manifest.name.trim()) {
    throw new Error("manifest field 'name' is required");
  }
  if (!NAME_RE.test(manifest.name)) {
    throw new Error(
      `invalid package name "${manifest.name}": must start with a letter and contain only letters, digits, '_' or '-'`,
    );
  }
  if (!manifest.version.trim()) {
    throw new Error("manifest field 'version' is required");
  }
  if (!VERSION_RE.test(manifest.version)) {
    throw new Error(
      `invalid version "${manifest.version}": expected semver like 1.2.3`,
    );
  }
  if (!manifest.author.trim()) {
    throw new Error("manifest field 'author' is required");
  }
  for (const [dep, ver] of Object.entries(manifest.dependencies ?? {})) {
    if (!NAME_RE.test(dep)) {
      throw new Error(`invalid dependency name "${dep}"`);
    }
    if (!String(ver).trim()) {
      throw new Error(`dependency "${dep}" has empty version`);
    }
  }
}

function errorMessage(err: unknown): string {
  if (err instanceof AxiosError) {
    return err.message;
  }
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}

function stringifyBody(data: unknown): string {
  if (typeof data === "string") {
    return data.trim();
  }
  try {
    return JSON.stringify(data);
  } catch {
    return String(data);
  }
}
