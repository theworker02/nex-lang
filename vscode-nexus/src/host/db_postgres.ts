/**
 * Postgres-backed registry store for the TypeScript Nexus host.
 * Active when DATABASE_URL is set. Uses SyncPg so db_* builtins stay sync.
 */

import * as crypto from "crypto";
import * as fs from "fs";
import * as path from "path";

import { MemoryDB, MemPackage, MemUser, MemVersion } from "./db_memory";
import { SyncPg } from "./sync_pg";

type Row = Record<string, unknown>;

function hashToken(token: string): string {
  return crypto.createHash("sha256").update(token).digest("hex");
}

function mapUser(row: Row): MemUser {
  const id = Number(row.id);
  const username = String(row.username ?? "");
  const email = String(row.email ?? "");
  const passwordHash = String(row.password_hash ?? "");
  const avatar = String(row.avatar_url ?? "");
  const bio = String(row.bio ?? "");
  const useGravatar = Boolean(row.use_gravatar);
  const githubLogin = String(row.github_login ?? "");
  const created = row.created_at
    ? new Date(String(row.created_at)).toISOString()
    : new Date().toISOString();
  const emailVerified = Boolean(row.email_verified);
  const totpEnabled = Boolean(row.totp_enabled);
  const isAdmin = Boolean(row.is_admin);
  return {
    id,
    ID: id,
    username,
    Username: username,
    email,
    Email: email,
    password_hash: passwordHash,
    avatar_url: avatar,
    AvatarURL: avatar,
    bio,
    Bio: bio,
    use_gravatar: useGravatar,
    UseGravatar: useGravatar,
    github_id: row.github_id == null ? null : Number(row.github_id),
    github_login: githubLogin,
    GitHubLogin: githubLogin,
    has_password: passwordHash.length > 0,
    HasPassword: passwordHash.length > 0,
    email_verified: emailVerified,
    EmailVerified: emailVerified,
    totp_enabled: totpEnabled,
    TOTPEnabled: totpEnabled,
    is_admin: isAdmin,
    IsAdmin: isAdmin,
    created_at: created,
    CreatedAt: created,
  };
}

function mapPackage(row: Row): MemPackage {
  const keywords = Array.isArray(row.keywords)
    ? (row.keywords as string[])
    : [];
  const categories = Array.isArray(row.categories)
    ? (row.categories as string[])
    : [];
  return {
    ID: Number(row.id),
    Name: String(row.name ?? ""),
    Description: String(row.description ?? ""),
    Author: String(row.author ?? ""),
    License: String(row.license ?? ""),
    Repository: String(row.repository ?? ""),
    Homepage: String(row.homepage ?? ""),
    Keywords: keywords,
    Categories: categories,
    Readme: String(row.readme ?? ""),
    DownloadCount: Number(row.download_count ?? 0),
    OwnerID: row.owner_id == null ? null : Number(row.owner_id),
    OwnerUsername: String(row.owner_username ?? row.author ?? ""),
    CreatedAt: row.created_at
      ? new Date(String(row.created_at)).toISOString()
      : new Date().toISOString(),
    UpdatedAt: row.updated_at
      ? new Date(String(row.updated_at)).toISOString()
      : new Date().toISOString(),
    LatestVersion: row.latest_version ? String(row.latest_version) : undefined,
  };
}

function mapVersion(row: Row): MemVersion {
  return {
    ID: Number(row.id),
    PackageID: Number(row.package_id),
    Version: String(row.version ?? ""),
    Checksum: String(row.checksum ?? ""),
    StoragePath: String(row.storage_path ?? ""),
    Filename: String(row.filename ?? ""),
    FileSize: Number(row.file_size ?? 0),
    ContentType: String(row.content_type ?? "application/octet-stream"),
    Yanked: Boolean(row.yanked),
    YankReason: String(row.yank_reason ?? ""),
    Deprecated: Boolean(row.deprecated),
    DeprecationMessage: String(row.deprecation_message ?? ""),
    CreatedAt: row.created_at
      ? new Date(String(row.created_at)).toISOString()
      : new Date().toISOString(),
  };
}

const PACKAGE_SELECT = `
  SELECT p.*,
    (SELECT v.version FROM versions v
      WHERE v.package_id = p.id AND v.yanked = FALSE
      ORDER BY v.created_at DESC LIMIT 1) AS latest_version,
    COALESCE(u.username, p.author, '') AS owner_username
  FROM packages p
  LEFT JOIN users u ON u.id = p.owner_id
`;

export class PostgresDB {
  readonly mode = "postgres";
  private pg: SyncPg;

  /** Compatibility mirrors used by /metrics and some host paths. */
  packages: MemPackage[] = [];
  versions: MemVersion[] = [];
  users: MemUser[] = [];
  reports: Array<Record<string, unknown>> = [];

  constructor(databaseUrl: string) {
    this.pg = new SyncPg(databaseUrl);
  }

  /** Apply SQL migration files from a directory (idempotent files expected). */
  applyMigrations(migrationsDir: string): void {
    if (!fs.existsSync(migrationsDir)) {
      return;
    }
    this.pg.exec(`
      CREATE TABLE IF NOT EXISTS schema_migrations (
        version TEXT PRIMARY KEY,
        applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
      )
    `);
    const files = fs
      .readdirSync(migrationsDir)
      .filter((f) => f.endsWith(".sql"))
      .sort();
    for (const file of files) {
      const version = file;
      const existing = this.pg.query(
        "SELECT 1 AS ok FROM schema_migrations WHERE version = $1",
        [version],
      );
      if (existing.rows.length) {
        continue;
      }
      const sql = fs.readFileSync(path.join(migrationsDir, file), "utf8");
      this.pg.exec(sql);
      this.pg.exec(
        "INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING",
        [version],
      );
      // eslint-disable-next-line no-console
      console.log(`[db] applied migration ${version}`);
    }
    // Optional columns used by TS host auth flows
    this.pg.exec(`
      ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE;
      ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_enabled BOOLEAN NOT NULL DEFAULT FALSE;
      ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;
      ALTER TABLE versions ADD COLUMN IF NOT EXISTS yank_reason TEXT NOT NULL DEFAULT '';
      ALTER TABLE versions ADD COLUMN IF NOT EXISTS deprecated BOOLEAN NOT NULL DEFAULT FALSE;
      ALTER TABLE versions ADD COLUMN IF NOT EXISTS deprecation_message TEXT NOT NULL DEFAULT '';
    `);
  }

  refreshCaches(): void {
    this.packages = this.pg
      .query(`${PACKAGE_SELECT} ORDER BY p.updated_at DESC`)
      .rows.map((r) => mapPackage(r));
    this.versions = this.pg
      .query("SELECT * FROM versions ORDER BY created_at DESC")
      .rows.map((r) => mapVersion(r));
    this.users = this.pg
      .query("SELECT * FROM users ORDER BY id")
      .rows.map((r) => mapUser(r));
  }

  seedFromStorage(storageDir: string): void {
    const count = this.pg.query("SELECT COUNT(*)::text AS c FROM packages");
    if (Number(count.rows[0]?.c ?? 0) > 0) {
      this.refreshCaches();
      return;
    }
    // Reuse MemoryDB seeder, then insert into Postgres
    const mem = new MemoryDB();
    mem.seedFromStorage(storageDir);
    for (const pkg of mem.packages) {
      const inserted = this.pg.query(
        `INSERT INTO packages (
          name, description, author, license, repository, homepage,
          keywords, categories, readme, download_count, created_at, updated_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
        RETURNING id`,
        [
          pkg.Name,
          pkg.Description,
          pkg.Author,
          pkg.License,
          pkg.Repository,
          pkg.Homepage,
          pkg.Keywords,
          pkg.Categories,
          pkg.Readme,
          pkg.DownloadCount,
        ],
      );
      const pkgId = Number(inserted.rows[0]?.id);
      for (const ver of mem.versions.filter((v) => v.PackageID === pkg.ID)) {
        this.pg.exec(
          `INSERT INTO versions (
            package_id, version, checksum, storage_path, filename, file_size,
            content_type, yanked, created_at
          ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW())
          ON CONFLICT (package_id, version) DO NOTHING`,
          [
            pkgId,
            ver.Version,
            ver.Checksum,
            ver.StoragePath,
            ver.Filename,
            ver.FileSize,
            ver.ContentType,
            ver.Yanked,
          ],
        );
      }
    }
    this.refreshCaches();
    // eslint-disable-next-line no-console
    console.log(
      `[db] seeded ${mem.packages.length} packages from storage into Postgres`,
    );
  }

  hubStats(): Record<string, unknown> {
    const row = this.pg.query(`SELECT
        (SELECT COUNT(*)::int FROM packages) AS packages,
        (SELECT COUNT(*)::int FROM versions) AS versions,
        (SELECT COALESCE(SUM(download_count),0)::bigint FROM packages) AS downloads,
        (SELECT COUNT(*)::int FROM users) AS users`).rows[0] as Row;
    return {
      Packages: Number(row.packages),
      Versions: Number(row.versions),
      Downloads: Number(row.downloads),
      Users: Math.max(Number(row.users), 1),
      Tags: this.topTags(16),
    };
  }

  topTags(limit: number): Array<{ Name: string; Count: number }> {
    const rows = this.pg.query(
      `SELECT tag AS name, COUNT(*)::int AS count FROM (
         SELECT UNNEST(keywords || categories) AS tag FROM packages
       ) t
       WHERE tag <> ''
       GROUP BY tag
       ORDER BY count DESC, tag ASC
       LIMIT $1`,
      [limit],
    ).rows;
    return rows.map((r) => ({
      Name: String(r.name),
      Count: Number(r.count),
    }));
  }

  topKeywords(
    limit: number,
  ): Array<{ Name: string; Count: number; Keyword?: string }> {
    return this.topTags(limit).map((t) => ({ ...t, Keyword: t.Name }));
  }

  listLicenses(
    limit: number,
  ): Array<{ Name: string; Count: number; License?: string }> {
    const rows = this.pg.query(
      `SELECT license AS name, COUNT(*)::int AS count
       FROM packages WHERE license <> ''
       GROUP BY license ORDER BY count DESC LIMIT $1`,
      [limit],
    ).rows;
    return rows.map((r) => ({
      Name: String(r.name),
      Count: Number(r.count),
      License: String(r.name),
    }));
  }

  listRecent(limit: number): MemPackage[] {
    return this.pg
      .query(`${PACKAGE_SELECT} ORDER BY p.updated_at DESC LIMIT $1`, [limit])
      .rows.map((r) => mapPackage(r));
  }

  listPopular(limit: number): MemPackage[] {
    return this.pg
      .query(`${PACKAGE_SELECT} ORDER BY p.download_count DESC LIMIT $1`, [
        limit,
      ])
      .rows.map((r) => mapPackage(r));
  }

  search(opts: Record<string, unknown>): {
    packages: MemPackage[];
    total: number;
    query: string;
    category: string;
    keyword: string;
    license: string;
    sort: string;
    limit: number;
    offset: number;
  } {
    const q = String(opts.q ?? opts.query ?? "").trim();
    const category = String(opts.category ?? "").trim();
    const keyword = String(opts.keyword ?? "").trim();
    const license = String(opts.license ?? "").trim();
    const sort = String(opts.sort ?? "relevance");
    const limit = Number(opts.limit ?? 50);
    const offset = Number(opts.offset ?? 0);
    const browse = Boolean(opts.browse);

    if (!browse && !q && !category && !keyword && !license) {
      return {
        packages: [],
        total: 0,
        query: q,
        category,
        keyword,
        license,
        sort,
        limit,
        offset,
      };
    }

    const wheres: string[] = [];
    const args: unknown[] = [];
    if (q) {
      args.push(`%${q.toLowerCase()}%`);
      wheres.push(`(LOWER(p.name) LIKE $${args.length} OR LOWER(p.description) LIKE $${args.length} OR EXISTS (
          SELECT 1 FROM UNNEST(p.keywords) k WHERE LOWER(k) LIKE $${args.length}
        ))`);
    }
    if (category) {
      args.push(category);
      wheres.push(`$${args.length} = ANY(p.categories)`);
    }
    if (keyword) {
      args.push(keyword);
      wheres.push(`$${args.length} = ANY(p.keywords)`);
    }
    if (license) {
      args.push(license);
      wheres.push(`p.license = $${args.length}`);
    }
    const whereSql = wheres.length ? wheres.join(" AND ") : "TRUE";

    let order = "p.name ASC";
    if (sort === "downloads") {
      order = "p.download_count DESC";
    } else if (sort === "recent" || sort === "updated") {
      order = "p.updated_at DESC";
    }

    const totalRow = this.pg.query(
      `SELECT COUNT(*)::int AS c FROM packages p WHERE ${whereSql}`,
      args,
    ).rows[0];
    const total = Number(totalRow?.c ?? 0);
    const limitIdx = args.length + 1;
    const offsetIdx = args.length + 2;
    const packages = this.pg
      .query(
        `${PACKAGE_SELECT} WHERE ${whereSql} ORDER BY ${order} LIMIT $${limitIdx} OFFSET $${offsetIdx}`,
        [...args, limit, offset],
      )
      .rows.map((r) => mapPackage(r));
    return {
      packages,
      total,
      query: q,
      category,
      keyword,
      license,
      sort,
      limit,
      offset,
    };
  }

  getPackage(name: string): MemPackage | null {
    const rows = this.pg.query(
      `${PACKAGE_SELECT} WHERE LOWER(p.name) = LOWER($1)`,
      [name],
    ).rows;
    return rows[0] ? mapPackage(rows[0]) : null;
  }

  listVersions(packageId: number): MemVersion[] {
    return this.pg
      .query(
        "SELECT * FROM versions WHERE package_id = $1 ORDER BY created_at DESC",
        [packageId],
      )
      .rows.map((r) => mapVersion(r));
  }

  getPackageVersion(
    name: string,
    version: string,
  ): { Package: MemPackage; Version: MemVersion } | null {
    const pkg = this.getPackage(name);
    if (!pkg) {
      return null;
    }
    const rows = this.pg.query(
      "SELECT * FROM versions WHERE package_id = $1 AND version = $2",
      [pkg.ID, version],
    ).rows;
    if (!rows[0]) {
      return null;
    }
    return { Package: pkg, Version: mapVersion(rows[0]) };
  }

  createUser(
    username: string,
    email: string,
    passwordHash: string,
    avatar = "",
    bio = "",
    useGravatar = false,
  ): MemUser {
    try {
      const rows = this.pg.query(
        `INSERT INTO users (username, email, password_hash, avatar_url, bio, use_gravatar)
         VALUES ($1,$2,$3,$4,$5,$6)
         RETURNING *`,
        [username, email, passwordHash, avatar, bio, useGravatar],
      ).rows;
      const u = mapUser(rows[0]!);
      this.users.push(u);
      return u;
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg.includes("unique") || msg.includes("duplicate")) {
        throw new Error("conflict: user already exists");
      }
      throw e;
    }
  }

  getUser(username: string): MemUser | null {
    const rows = this.pg.query(
      `SELECT * FROM users
       WHERE LOWER(username) = LOWER($1) OR LOWER(email) = LOWER($1)
       LIMIT 1`,
      [username],
    ).rows;
    return rows[0] ? mapUser(rows[0]) : null;
  }

  getUserById(id: number): MemUser | null {
    const rows = this.pg.query("SELECT * FROM users WHERE id = $1", [id]).rows;
    return rows[0] ? mapUser(rows[0]) : null;
  }

  createSession(userId: number): string {
    const token = "nxs_" + crypto.randomBytes(24).toString("hex");
    const tokenHash = hashToken(token);
    this.pg.exec(
      `INSERT INTO sessions (user_id, token_hash, expires_at)
       VALUES ($1, $2, NOW() + INTERVAL '30 days')`,
      [userId, tokenHash],
    );
    return token;
  }

  userFromSession(token: string): MemUser | null {
    if (!token) {
      return null;
    }
    const tokenHash = hashToken(token);
    const rows = this.pg.query(
      `SELECT u.* FROM sessions s
       JOIN users u ON u.id = s.user_id
       WHERE s.token_hash = $1 AND s.expires_at > NOW()
       LIMIT 1`,
      [tokenHash],
    ).rows;
    return rows[0] ? mapUser(rows[0]) : null;
  }

  deleteSession(token: string): void {
    if (!token) {
      return;
    }
    this.pg.exec("DELETE FROM sessions WHERE token_hash = $1", [
      hashToken(token),
    ]);
  }

  incrementDownloads(packageId: number): void {
    this.pg.exec(
      "UPDATE packages SET download_count = download_count + 1, updated_at = NOW() WHERE id = $1",
      [packageId],
    );
  }

  updateProfile(
    id: number,
    bio: string,
    avatar: string,
    useGravatar: boolean,
  ): MemUser | null {
    const rows = this.pg.query(
      `UPDATE users SET bio = $2, avatar_url = $3, use_gravatar = $4, updated_at = NOW()
       WHERE id = $1 RETURNING *`,
      [id, bio, avatar, useGravatar],
    ).rows;
    return rows[0] ? mapUser(rows[0]) : null;
  }

  createAuthToken(userId: number, purpose: string, ttlSec: number): string {
    const token = "tok_" + crypto.randomBytes(24).toString("hex");
    this.pg.exec(
      `INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at)
       VALUES ($1, $2, $3, NOW() + make_interval(secs => $4))`,
      [userId, purpose, hashToken(token), ttlSec],
    );
    return token;
  }

  consumeAuthToken(token: string, purpose: string): MemUser | null {
    const rows = this.pg.query(
      `UPDATE auth_tokens SET used_at = NOW()
       WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > NOW()
       RETURNING user_id`,
      [hashToken(token), purpose],
    ).rows;
    if (!rows[0]) {
      return null;
    }
    return this.getUserById(Number(rows[0].user_id));
  }

  peekAuthToken(token: string, purpose: string): MemUser | null {
    const rows = this.pg.query(
      `SELECT user_id FROM auth_tokens
       WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > NOW()`,
      [hashToken(token), purpose],
    ).rows;
    if (!rows[0]) {
      return null;
    }
    return this.getUserById(Number(rows[0].user_id));
  }

  markEmailVerified(userId: number): void {
    this.pg.exec(
      "UPDATE users SET email_verified = TRUE, updated_at = NOW() WHERE id = $1",
      [userId],
    );
  }

  setPassword(userId: number, passwordHash: string): void {
    this.pg.exec(
      "UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1",
      [userId, passwordHash],
    );
  }

  createAbuseReport(data: Record<string, unknown>): Record<string, unknown> {
    // Prefer a reports table if present; otherwise keep in-process.
    try {
      const rows = this.pg.query(
        `INSERT INTO abuse_reports (category, details, reporter_email, package_name, status, created_at)
         VALUES ($1,$2,$3,$4,'open',NOW()) RETURNING id`,
        [
          String(data.Category ?? data.category ?? ""),
          String(data.Details ?? data.details ?? data.Message ?? ""),
          String(data.Email ?? data.email ?? ""),
          String(data.Package ?? data.package ?? data.PackageName ?? ""),
        ],
      ).rows;
      const row = { ID: Number(rows[0]?.id), ...data, Status: "open" };
      this.reports.push(row);
      return row;
    } catch {
      const row = { ID: this.reports.length + 1, ...data, Status: "open" };
      this.reports.push(row);
      return row;
    }
  }

  listPackageOwners(packageId: number): Array<Record<string, unknown>> {
    return this.pg.query(
      `SELECT po.*, u.username
       FROM package_owners po
       LEFT JOIN users u ON u.id = po.user_id
       WHERE po.package_id = $1 AND (po.accepted_at IS NOT NULL OR po.invite_token_hash IS NULL)`,
      [packageId],
    ).rows.map((r) => ({
      ID: Number(r.id),
      PackageID: packageId,
      UserID: r.user_id == null ? null : Number(r.user_id),
      Username: String(r.username ?? ""),
      Role: String(r.role ?? "maintainer"),
      OrgID: r.org_id == null ? null : Number(r.org_id),
    }));
  }

  packageOwnerRole(packageId: number, userId: number): string {
    const rows = this.pg.query(
      `SELECT role FROM package_owners
       WHERE package_id = $1 AND user_id = $2 AND accepted_at IS NOT NULL
       LIMIT 1`,
      [packageId, userId],
    ).rows;
    return rows[0] ? String(rows[0].role) : "";
  }

  listPackagesByOwner(userId: number): MemPackage[] {
    return this.pg
      .query(
        `${PACKAGE_SELECT}
         WHERE p.owner_id = $1
            OR EXISTS (
              SELECT 1 FROM package_owners po
              WHERE po.package_id = p.id AND po.user_id = $1 AND po.accepted_at IS NOT NULL
            )
         ORDER BY p.updated_at DESC`,
        [userId],
      )
      .rows.map((r) => mapPackage(r));
  }

  userStats(userId: number): Record<string, unknown> {
    const pkgs = this.listPackagesByOwner(userId);
    const downloads = pkgs.reduce((a, p) => a + p.DownloadCount, 0);
    const versions = this.pg.query(
      `SELECT COUNT(*)::int AS c FROM versions v
       JOIN packages p ON p.id = v.package_id
       WHERE p.owner_id = $1`,
      [userId],
    ).rows[0];
    return {
      PackageCount: pkgs.length,
      VersionCount: Number(versions?.c ?? 0),
      TotalDownloads: downloads,
      APIKeyCount: Number(
        this.pg.query(
          "SELECT COUNT(*)::int AS c FROM api_keys WHERE user_id = $1 AND revoked_at IS NULL",
          [userId],
        ).rows[0]?.c ?? 0,
      ),
      TrustedCount: Number(
        this.pg.query(
          "SELECT COUNT(*)::int AS c FROM trusted_publishers WHERE user_id = $1",
          [userId],
        ).rows[0]?.c ?? 0,
      ),
      package_count: pkgs.length,
      version_count: Number(versions?.c ?? 0),
      total_downloads: downloads,
    };
  }

  listApiKeys(userId: number): Array<Record<string, unknown>> {
    return this.pg
      .query(
        `SELECT id, name, prefix, created_at, last_used_at, revoked_at
         FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`,
        [userId],
      )
      .rows.map((r) => ({
        ID: Number(r.id),
        Name: String(r.name ?? ""),
        Prefix: String(r.prefix ?? ""),
        CreatedAt: r.created_at,
        LastUsedAt: r.last_used_at,
        RevokedAt: r.revoked_at,
      }));
  }

  createApiKey(
    userId: number,
    name: string,
    _scope?: string,
    _expiresDays?: number,
  ): Record<string, unknown> {
    const raw = "nex_" + crypto.randomBytes(24).toString("hex");
    const prefix = raw.slice(0, 12);
    const keyHash = hashToken(raw);
    const rows = this.pg.query(
      `INSERT INTO api_keys (user_id, name, prefix, key_hash)
       VALUES ($1,$2,$3,$4) RETURNING id, created_at`,
      [userId, name, prefix, keyHash],
    ).rows;
    return {
      ID: Number(rows[0]?.id),
      Name: name,
      Prefix: prefix,
      Key: raw,
      token: raw,
      CreatedAt: rows[0]?.created_at,
    };
  }

  revokeApiKey(userId: number, keyId: number): void {
    this.pg.exec(
      "UPDATE api_keys SET revoked_at = NOW() WHERE id = $1 AND user_id = $2",
      [keyId, userId],
    );
  }

  listTrusted(userId: number): Array<Record<string, unknown>> {
    return this.pg
      .query(
        "SELECT * FROM trusted_publishers WHERE user_id = $1 ORDER BY created_at DESC",
        [userId],
      )
      .rows.map((r) => ({
        ID: Number(r.id),
        Provider: String(r.provider ?? ""),
        RepositoryOwner: String(r.repository_owner ?? ""),
        RepositoryName: String(r.repository_name ?? ""),
        WorkflowFilename: String(r.workflow_filename ?? ""),
        Environment: String(r.environment ?? ""),
        PackageScope: String(r.package_scope ?? ""),
      }));
  }

  getOrg(slug: string): Record<string, unknown> | null {
    const rows = this.pg.query(
      "SELECT * FROM organizations WHERE LOWER(slug) = LOWER($1)",
      [slug],
    ).rows;
    if (!rows[0]) {
      return null;
    }
    const r = rows[0];
    return {
      ID: Number(r.id),
      Slug: String(r.slug),
      DisplayName: String(r.display_name ?? r.slug),
      Description: String(r.description ?? ""),
      AvatarURL: String(r.avatar_url ?? ""),
      CreatedAt: r.created_at,
    };
  }

  createOrg(
    slug: string,
    displayName: string,
    description: string,
    userId: number,
  ): Record<string, unknown> | null {
    const rows = this.pg.query(
      `INSERT INTO organizations (slug, display_name, description, created_by_user_id)
       VALUES ($1,$2,$3,$4) RETURNING *`,
      [slug, displayName, description, userId],
    ).rows;
    this.pg.exec(
      `INSERT INTO organization_members (org_id, user_id, role)
       VALUES ($1,$2,'owner')`,
      [rows[0]!.id, userId],
    );
    return this.getOrg(slug);
  }

  listUserOrgs(userId: number): Array<Record<string, unknown>> {
    return this.pg
      .query(
        `SELECT o.* FROM organizations o
         JOIN organization_members m ON m.org_id = o.id
         WHERE m.user_id = $1 ORDER BY o.slug`,
        [userId],
      )
      .rows.map((r) => ({
        ID: Number(r.id),
        Slug: String(r.slug),
        DisplayName: String(r.display_name ?? r.slug),
        Description: String(r.description ?? ""),
      }));
  }

  close(): void {
    this.pg.close();
  }
}
