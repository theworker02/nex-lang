export class NexApiError extends Error {
  readonly status: number;
  readonly payload?: unknown;

  constructor(message: string, status: number, payload?: unknown) {
    super(message);
    this.name = "NexApiError";
    this.status = status;
    this.payload = payload;
  }
}

export interface NexClientOptions {
  baseUrl?: string;
  token?: string;
  fetchImpl?: typeof fetch;
}

export interface NexUser {
  id: number;
  username: string;
  email: string;
}

export interface LoginResult {
  message: string;
  user: NexUser;
  token: string;
}

export interface VersionInfo {
  version: string;
  checksum: string;
  filename: string;
  yanked: boolean;
  reason: string;
}

export interface PublishResult {
  message: string;
  checksum: string;
  filename: string;
  downloadUrl: string;
  packageName: string;
  version: string;
}

export interface YankResult {
  message: string;
  name: string;
  version: string;
  reason: string;
}

export interface PublishInput {
  manifest: Blob;
  package: Blob;
  readme?: Blob;
}

function trimSlash(url: string): string {
  return url.replace(/\/+$/, "");
}

function firstString(obj: Record<string, unknown>, ...keys: string[]): string {
  for (const key of keys) {
    const value = obj[key];
    if (typeof value === "string" && value) return value;
  }
  return "";
}

function asRecord(value: unknown): Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function versionFromMap(raw: Record<string, unknown>): VersionInfo {
  return {
    version: firstString(raw, "Version", "version"),
    checksum: firstString(raw, "Checksum", "checksum"),
    filename: firstString(raw, "Filename", "filename"),
    yanked: raw.Yanked === true || raw.yanked === true,
    reason: firstString(raw, "YankReason", "yank_reason", "Reason", "reason"),
  };
}

/**
 * Typed client for the Nexus registry REST API (login, keys, resolve, download, publish, yank).
 */
export class NexClient {
  readonly baseUrl: string;
  private token: string;
  private readonly fetchImpl: typeof fetch;

  constructor(opts: NexClientOptions = {}) {
    this.baseUrl = trimSlash(opts.baseUrl ?? "http://localhost:8080");
    this.token = opts.token ?? "";
    this.fetchImpl = opts.fetchImpl ?? fetch;
  }

  setToken(token: string): void {
    this.token = token.trim();
  }

  getToken(): string {
    return this.token;
  }

  async login(login: string, password: string): Promise<LoginResult> {
    const result = await this.request<LoginResult>("POST", "/api/auth/login", {
      json: { login, password },
      auth: false,
    });
    if (!result.token) throw new NexApiError("login response missing token", 200, result);
    this.token = result.token;
    return result;
  }

  async profile(): Promise<NexUser> {
    const wrap = await this.request<{ user: NexUser }>("GET", "/api/user/profile");
    return wrap.user;
  }

  async createApiKey(name = "sdk"): Promise<string> {
    const wrap = await this.request<{ api_key: string }>("POST", "/api/user/api-keys", {
      json: { name },
    });
    if (!wrap.api_key) throw new NexApiError("api key response missing api_key", 200, wrap);
    return wrap.api_key;
  }

  async resolvePackage(name: string, version = ""): Promise<VersionInfo> {
    if (!name) throw new Error("package name is required");
    if (!version) return this.resolveLatest(name);
    const wrap = await this.request<{ version?: Record<string, unknown> }>(
      "GET",
      `/api/v1/packages/${encodeURIComponent(name)}/${encodeURIComponent(version)}`,
      { auth: false },
    );
    const info = versionFromMap(asRecord(wrap.version));
    if (!info.version) info.version = version;
    if (!info.checksum) {
      throw new Error(`registry response for ${name}@${version} is missing checksum`);
    }
    return info;
  }

  async downloadPackage(name: string, version: string): Promise<ArrayBuffer> {
    if (!name || !version) throw new Error("package name and version are required");
    const path = `/api/v1/packages/${encodeURIComponent(name)}/${encodeURIComponent(version)}/download`;
    const res = await this.fetchImpl(`${this.baseUrl}${path}`, {
      headers: this.headers({ auth: false, json: false }),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new NexApiError(`download failed for ${name}@${version}`, res.status, text);
    }
    return res.arrayBuffer();
  }

  async publish(input: PublishInput): Promise<PublishResult> {
    this.requireAuth();
    const form = new FormData();
    form.set("nexus.toml", input.manifest, "nexus.toml");
    form.set("package", input.package, "package.nex");
    if (input.readme) form.set("readme", input.readme, "README.md");

    const res = await this.fetchImpl(`${this.baseUrl}/api/v1/publish`, {
      method: "POST",
      headers: this.headers({ json: false }),
      body: form,
    });
    const raw = asRecord(await this.parseJson(res));
    return {
      message: String(raw.message ?? ""),
      checksum: firstString(raw, "checksum") || firstString(asRecord(raw.version), "Checksum", "checksum"),
      filename: firstString(raw, "filename") || firstString(asRecord(raw.version), "Filename", "filename"),
      downloadUrl: String(raw.download_url ?? ""),
      packageName: firstString(asRecord(raw.package), "Name", "name"),
      version: firstString(asRecord(raw.version), "Version", "version"),
    };
  }

  async yank(name: string, version: string, reason: string): Promise<YankResult> {
    this.requireAuth();
    const trimmed = reason.trim();
    if (!name || !version) throw new Error("package name and version are required");
    if (!trimmed) throw new Error("yank reason is required");
    const raw = asRecord(
      await this.request<unknown>(
        "POST",
        `/api/v1/packages/${encodeURIComponent(name)}/${encodeURIComponent(version)}/yank`,
        { json: { reason: trimmed } },
      ),
    );
    return {
      message: String(raw.message ?? ""),
      name: firstString(asRecord(raw.package), "Name", "name") || name,
      version: firstString(asRecord(raw.version), "Version", "version") || version,
      reason: firstString(asRecord(raw.version), "YankReason", "yank_reason") || trimmed,
    };
  }

  private async resolveLatest(name: string): Promise<VersionInfo> {
    const wrap = await this.request<{ versions?: unknown }>(
      "GET",
      `/api/v1/packages/${encodeURIComponent(name)}`,
      { auth: false },
    );
    const versions = Array.isArray(wrap.versions) ? wrap.versions : [];
    for (const raw of versions) {
      const info = versionFromMap(asRecord(raw));
      if (info.version && info.checksum && !info.yanked) return info;
    }
    throw new Error(`package ${JSON.stringify(name)} has no installable (non-yanked) versions`);
  }

  private requireAuth(): void {
    if (!this.token) throw new Error("authentication required: login() or setToken()");
  }

  private async request<T>(
    method: string,
    path: string,
    opts: { json?: unknown; auth?: boolean } = {},
  ): Promise<T> {
    const headers = this.headers({ auth: opts.auth, json: opts.json !== undefined });
    const res = await this.fetchImpl(`${this.baseUrl}${path}`, {
      method,
      headers,
      body: opts.json !== undefined ? JSON.stringify(opts.json) : undefined,
    });
    return (await this.parseJson(res)) as T;
  }

  private async parseJson(res: Response): Promise<unknown> {
    const text = await res.text();
    let payload: unknown = text;
    try {
      payload = text ? JSON.parse(text) : undefined;
    } catch {
      payload = text;
    }
    if (!res.ok) {
      const message =
        typeof payload === "object" && payload !== null && "error" in payload
          ? String((payload as { error: unknown }).error)
          : text || `HTTP ${res.status}`;
      throw new NexApiError(message, res.status, payload);
    }
    return payload;
  }

  private headers(opts: { auth?: boolean; json?: boolean } = {}): Headers {
    const headers = new Headers();
    headers.set("accept", "application/json");
    if (opts.json !== false) headers.set("content-type", "application/json");
    if (opts.auth !== false && this.token) headers.set("authorization", `Bearer ${this.token}`);
    return headers;
  }
}

export default NexClient;
