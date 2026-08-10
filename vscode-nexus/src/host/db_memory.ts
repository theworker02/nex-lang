/**
 * In-memory registry store for demo / static mode (no Postgres required).
 */

import * as crypto from "crypto";
import * as fs from "fs";
import * as path from "path";

export interface MemPackage {
  ID: number;
  Name: string;
  Description: string;
  Author: string;
  License: string;
  Repository: string;
  Homepage: string;
  Keywords: string[];
  Categories: string[];
  Readme: string;
  DownloadCount: number;
  OwnerID: number | null;
  OwnerUsername: string;
  CreatedAt: string;
  UpdatedAt: string;
  LatestVersion?: string;
}

export interface MemVersion {
  ID: number;
  PackageID: number;
  Version: string;
  Checksum: string;
  StoragePath: string;
  Filename: string;
  FileSize: number;
  ContentType: string;
  Yanked: boolean;
  YankReason: string;
  Deprecated: boolean;
  DeprecationMessage: string;
  CreatedAt: string;
}

export interface MemUser {
  id: number;
  ID: number;
  username: string;
  Username: string;
  email: string;
  Email: string;
  password_hash: string;
  avatar_url: string;
  AvatarURL: string;
  bio: string;
  Bio: string;
  use_gravatar: boolean;
  UseGravatar: boolean;
  github_id: number | null;
  github_login: string;
  GitHubLogin: string;
  has_password: boolean;
  HasPassword: boolean;
  email_verified: boolean;
  EmailVerified: boolean;
  totp_enabled: boolean;
  TOTPEnabled: boolean;
  is_admin: boolean;
  IsAdmin: boolean;
  created_at: string;
  CreatedAt: string;
}

export class MemoryDB {
  packages: MemPackage[] = [];
  versions: MemVersion[] = [];
  users: MemUser[] = [];
  sessions = new Map<string, number>();
  apiKeys: Array<Record<string, unknown>> = [];
  trusted: Array<Record<string, unknown>> = [];
  audit: Array<Record<string, unknown>> = [];
  reports: Array<Record<string, unknown>> = [];
  deps = new Map<number, Array<Record<string, unknown>>>();
  owners = new Map<number, Array<Record<string, unknown>>>();
  orgs: Array<Record<string, unknown>> = [];
  nextPkg = 1;
  nextVer = 1;
  nextUser = 1;

  seedFromStorage(storageDir: string): void {
    if (!fs.existsSync(storageDir)) {
      this.seedDefaults();
      return;
    }
    const now = new Date().toISOString();
    let found = false;
    for (const name of fs.readdirSync(storageDir)) {
      const pkgDir = path.join(storageDir, name);
      if (!fs.statSync(pkgDir).isDirectory()) {
        continue;
      }
      for (const ver of fs.readdirSync(pkgDir)) {
        const verDir = path.join(pkgDir, ver);
        if (!fs.statSync(verDir).isDirectory()) {
          continue;
        }
        const files = fs.readdirSync(verDir).filter((f) => f.endsWith(".nex"));
        if (files.length === 0) {
          continue;
        }
        const filename = files[0]!;
        const storagePath = path.join(verDir, filename);
        const data = fs.readFileSync(storagePath);
        const checksum =
          "sha256:" + crypto.createHash("sha256").update(data).digest("hex");
        let pkg = this.packages.find((p) => p.Name === name);
        if (!pkg) {
          pkg = {
            ID: this.nextPkg++,
            Name: name,
            Description: `${name} package for Nexus`,
            Author: "nexus",
            License: "MIT",
            Repository: "",
            Homepage: "",
            Keywords: [name, "nex"],
            Categories: name === "httpkit" ? ["network"] : ["web"],
            Readme: `# ${name}\n\nDemo package seeded from \`storage/\`.\n\nInstall with \`nex install ${name}@${ver}\`.\n`,
            DownloadCount: 42,
            OwnerID: null,
            OwnerUsername: "nexus",
            CreatedAt: now,
            UpdatedAt: now,
            LatestVersion: ver,
          };
          this.packages.push(pkg);
        }
        const version: MemVersion = {
          ID: this.nextVer++,
          PackageID: pkg.ID,
          Version: ver,
          Checksum: checksum,
          StoragePath: storagePath,
          Filename: filename,
          FileSize: data.length,
          ContentType: "application/octet-stream",
          Yanked: false,
          YankReason: "",
          Deprecated: false,
          DeprecationMessage: "",
          CreatedAt: now,
        };
        this.versions.push(version);
        pkg.LatestVersion = ver;
        pkg.UpdatedAt = now;
        found = true;
      }
    }
    if (!found) {
      this.seedDefaults();
    }
  }

  private seedDefaults(): void {
    const now = new Date().toISOString();
    const pkg: MemPackage = {
      ID: this.nextPkg++,
      Name: "httpkit",
      Description: "Tiny HTTP helpers for Nexus",
      Author: "nexus",
      License: "MIT",
      Repository: "",
      Homepage: "",
      Keywords: ["http", "net"],
      Categories: ["network"],
      Readme: "# httpkit\n\nTiny HTTP helpers for Nexus apps.\n",
      DownloadCount: 128,
      OwnerID: null,
      OwnerUsername: "nexus",
      CreatedAt: now,
      UpdatedAt: now,
      LatestVersion: "1.0.0",
    };
    this.packages.push(pkg);
    this.versions.push({
      ID: this.nextVer++,
      PackageID: pkg.ID,
      Version: "1.0.0",
      Checksum: "sha256:demo",
      StoragePath: "",
      Filename: "httpkit-1.0.0.nex",
      FileSize: 32,
      ContentType: "application/octet-stream",
      Yanked: false,
      YankReason: "",
      Deprecated: false,
      DeprecationMessage: "",
      CreatedAt: now,
    });
  }

  hubStats(): Record<string, unknown> {
    return {
      Packages: this.packages.length,
      Versions: this.versions.length,
      Downloads: this.packages.reduce((a, p) => a + p.DownloadCount, 0),
      Users: Math.max(this.users.length, 1),
      Tags: this.topTags(16),
    };
  }

  topTags(limit: number): Array<{ Name: string; Count: number }> {
    const counts = new Map<string, number>();
    for (const p of this.packages) {
      for (const t of [...p.Keywords, ...p.Categories]) {
        counts.set(t, (counts.get(t) ?? 0) + 1);
      }
    }
    return [...counts.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, limit)
      .map(([Name, Count]) => ({ Name, Count }));
  }

  topKeywords(limit: number): Array<{ Name: string; Count: number; Keyword?: string }> {
    return this.topTags(limit).map((t) => ({ ...t, Keyword: t.Name }));
  }

  listLicenses(limit: number): Array<{ Name: string; Count: number; License?: string }> {
    const counts = new Map<string, number>();
    for (const p of this.packages) {
      if (p.License) {
        counts.set(p.License, (counts.get(p.License) ?? 0) + 1);
      }
    }
    return [...counts.entries()]
      .slice(0, limit)
      .map(([Name, Count]) => ({ Name, Count, License: Name }));
  }

  listRecent(limit: number): MemPackage[] {
    return [...this.packages]
      .sort((a, b) => (a.UpdatedAt < b.UpdatedAt ? 1 : -1))
      .slice(0, limit)
      .map((p) => ({ ...p }));
  }

  listPopular(limit: number): MemPackage[] {
    return [...this.packages]
      .sort((a, b) => b.DownloadCount - a.DownloadCount)
      .slice(0, limit)
      .map((p) => ({ ...p }));
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
    const q = String(opts.q ?? opts.query ?? "").toLowerCase();
    const category = String(opts.category ?? "").toLowerCase();
    const keyword = String(opts.keyword ?? "").toLowerCase();
    const license = String(opts.license ?? "");
    const sort = String(opts.sort ?? "relevance");
    const limit = Number(opts.limit ?? 50);
    const offset = Number(opts.offset ?? 0);
    const browse = Boolean(opts.browse);

    let rows = [...this.packages];
    if (q) {
      rows = rows.filter(
        (p) =>
          p.Name.toLowerCase().includes(q) ||
          p.Description.toLowerCase().includes(q) ||
          p.Keywords.some((k) => k.toLowerCase().includes(q)),
      );
    }
    if (category) {
      rows = rows.filter((p) =>
        p.Categories.some((c) => c.toLowerCase() === category),
      );
    }
    if (keyword) {
      rows = rows.filter((p) =>
        p.Keywords.some((k) => k.toLowerCase() === keyword),
      );
    }
    if (license) {
      rows = rows.filter((p) => p.License === license);
    }
    if (!browse && !q && !category && !keyword && !license) {
      rows = [];
    }

    if (sort === "downloads") {
      rows.sort((a, b) => b.DownloadCount - a.DownloadCount);
    } else if (sort === "recent") {
      rows.sort((a, b) => (a.UpdatedAt < b.UpdatedAt ? 1 : -1));
    } else {
      rows.sort((a, b) => a.Name.localeCompare(b.Name));
    }

    const total = rows.length;
    const packages = rows.slice(offset, offset + limit).map((p) => ({ ...p }));
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
    return this.packages.find((p) => p.Name === name) ?? null;
  }

  listVersions(packageId: number): MemVersion[] {
    return this.versions
      .filter((v) => v.PackageID === packageId)
      .sort((a, b) => (a.CreatedAt < b.CreatedAt ? 1 : -1));
  }

  getPackageVersion(
    name: string,
    version: string,
  ): { Package: MemPackage; Version: MemVersion } | null {
    const pkg = this.getPackage(name);
    if (!pkg) {
      return null;
    }
    const ver = this.versions.find(
      (v) => v.PackageID === pkg.ID && v.Version === version,
    );
    if (!ver) {
      return null;
    }
    return { Package: pkg, Version: ver };
  }

  createUser(
    username: string,
    email: string,
    passwordHash: string,
    avatar = "",
    bio = "",
    useGravatar = false,
  ): MemUser {
    const now = new Date().toISOString();
    const id = this.nextUser++;
    const u: MemUser = {
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
      github_id: null,
      github_login: "",
      GitHubLogin: "",
      has_password: true,
      HasPassword: true,
      email_verified: false,
      EmailVerified: false,
      totp_enabled: false,
      TOTPEnabled: false,
      is_admin: false,
      IsAdmin: false,
      created_at: now,
      CreatedAt: now,
    };
    this.users.push(u);
    return u;
  }

  getUser(username: string): MemUser | null {
    return (
      this.users.find(
        (u) =>
          u.username === username ||
          u.email.toLowerCase() === username.toLowerCase(),
      ) ?? null
    );
  }

  getUserById(id: number): MemUser | null {
    return this.users.find((u) => u.id === id) ?? null;
  }

  createSession(userId: number): string {
    const token = "nxs_" + crypto.randomBytes(24).toString("hex");
    this.sessions.set(token, userId);
    return token;
  }

  userFromSession(token: string): MemUser | null {
    const id = this.sessions.get(token);
    if (id === undefined) {
      return null;
    }
    return this.getUserById(id);
  }

  deleteSession(token: string): void {
    this.sessions.delete(token);
  }
}
