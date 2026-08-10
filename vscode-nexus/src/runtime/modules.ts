/**
 * Module resolution for Nexus imports: local paths, `.modules/`, and stdlib.
 */

import * as fs from "fs";
import * as path from "path";

export const MODULES_DIRNAME = ".modules";

export interface ModuleResolveOptions {
  rootDir: string;
  modulesDir?: string;
  stdlibDir?: string;
  /** Directory of the importing file (`__dir__`). */
  fromDir?: string;
}

export function findStdlibDir(fromDir: string): string {
  const candidates: string[] = [
    path.join(fromDir, "stdlib"),
    path.join(fromDir, "..", "stdlib"),
  ];
  let dir = fromDir;
  for (let i = 0; i < 8; i++) {
    candidates.push(path.join(dir, "stdlib"));
    // vscode-nexus/stdlib when rooted at extension
    candidates.push(path.join(dir, "vscode-nexus", "stdlib"));
    const parent = path.dirname(dir);
    if (parent === dir) {
      break;
    }
    dir = parent;
  }
  for (const c of candidates) {
    try {
      const abs = path.resolve(c);
      if (fs.statSync(abs).isDirectory()) {
        return abs;
      }
    } catch {
      // continue
    }
  }
  return "";
}

function expandModuleCandidates(candidate: string): string[] {
  const ext = path.extname(candidate);
  if (ext === ".nex") {
    return [candidate];
  }
  return [
    `${candidate}.nex`,
    path.join(candidate, "mod.nex"),
    path.join(candidate, "main.nex"),
  ];
}

/**
 * Resolve an import path against the importer directory, app root,
 * `.modules/`, and stdlib.
 */
export function resolveModulePath(
  importPath: string,
  options: ModuleResolveOptions,
): string | null {
  const trimmed = importPath.trim().replace(/\\/g, "/");
  if (!trimmed) {
    return null;
  }

  const modulesDir =
    options.modulesDir ?? path.join(options.rootDir, MODULES_DIRNAME);
  const stdlibDir =
    options.stdlibDir !== undefined
      ? options.stdlibDir
      : findStdlibDir(options.rootDir);

  const candidates: string[] = [];
  if (path.isAbsolute(trimmed)) {
    candidates.push(trimmed);
  } else {
    const base = options.fromDir || options.rootDir;
    candidates.push(path.join(base, trimmed));
    candidates.push(path.join(options.rootDir, trimmed));
    candidates.push(path.join(modulesDir, trimmed));
    if (stdlibDir) {
      candidates.push(path.join(stdlibDir, trimmed));
      if (trimmed.startsWith("std/") || trimmed.startsWith("std" + path.sep)) {
        candidates.push(path.join(stdlibDir, trimmed.slice(4)));
      }
    }
  }

  const tried = new Set<string>();
  for (const c of candidates) {
    for (const full of expandModuleCandidates(c)) {
      const cleaned = path.normalize(full);
      if (tried.has(cleaned)) {
        continue;
      }
      tried.add(cleaned);
      try {
        if (fs.statSync(cleaned).isFile()) {
          return cleaned;
        }
      } catch {
        // continue
      }
    }
  }
  return null;
}

export function readModuleSource(absolutePath: string): string {
  return fs.readFileSync(absolutePath, "utf8");
}
