/**
 * Discover and run Nexus test files (*_test.nex and tests/*.nex).
 */

import * as fs from "fs";
import * as path from "path";
import { evaluate } from "./compiler/engine";
import { findStdlibDir, MODULES_DIRNAME } from "./runtime/modules";
import { isError } from "./language/values";

export interface TestResult {
  file: string;
  passed: boolean;
  durationMs: number;
  error?: string;
  output: string[];
}

export interface TestSummary {
  results: TestResult[];
  passed: number;
  failed: number;
}

export interface TestOptions {
  rootDir: string;
  paths?: string[];
  stdlibDir?: string;
  out?: (line: string) => void;
}

function discover(root: string, paths: string[]): string[] {
  const files: string[] = [];
  const seen = new Set<string>();
  const add = (p: string) => {
    const cleaned = path.normalize(p);
    if (seen.has(cleaned)) {
      return;
    }
    seen.add(cleaned);
    files.push(cleaned);
  };

  const searchRoots =
    paths.length > 0 ? paths : [path.join(root, "tests"), root];

  for (let p of searchRoots) {
    if (!path.isAbsolute(p)) {
      p = path.join(root, p);
    }
    let st: fs.Stats;
    try {
      st = fs.statSync(p);
    } catch {
      continue;
    }
    if (!st.isDirectory()) {
      if (p.endsWith(".nex")) {
        add(p);
      }
      continue;
    }
    walk(p, root, add);
  }
  return files;
}

function walk(
  dir: string,
  root: string,
  add: (p: string) => void,
): void {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (
        entry.name === "node_modules" ||
        entry.name === ".git" ||
        entry.name === "out" ||
        entry.name === "bin"
      ) {
        continue;
      }
      walk(full, root, add);
      continue;
    }
    const rel = path.relative(root, full).replace(/\\/g, "/");
    const inTests = rel.startsWith("tests/");
    if (
      entry.name.endsWith("_test.nex") ||
      (inTests && entry.name.endsWith(".nex"))
    ) {
      add(full);
    }
  }
}

export async function runTests(opts: TestOptions): Promise<TestSummary> {
  const log = opts.out ?? ((line: string) => {
    // eslint-disable-next-line no-console
    console.log(line);
  });
  const files = discover(opts.rootDir, opts.paths ?? []);
  if (files.length === 0) {
    throw new Error(
      "no test files found (looked for *_test.nex and tests/**/*.nex)",
    );
  }

  const stdlibDir =
    opts.stdlibDir !== undefined
      ? opts.stdlibDir
      : findStdlibDir(opts.rootDir);
  const summary: TestSummary = { results: [], passed: 0, failed: 0 };

  for (const file of files) {
    const start = Date.now();
    const source = fs.readFileSync(file, "utf8");
    const result = await evaluate(source, {
      tier: "eval",
      rootDir: opts.rootDir,
      modulesDir: path.join(opts.rootDir, MODULES_DIRNAME),
      stdlibDir,
      filePath: file,
      checkOwnership: false,
      enableEffects: false,
    });
    const durationMs = Date.now() - start;
    const rel = path.relative(opts.rootDir, file) || file;
    const failed = isError(result.value);
    const tr: TestResult = {
      file,
      passed: !failed,
      durationMs,
      error: failed ? result.value.inspect() : undefined,
      output: result.output,
    };
    summary.results.push(tr);
    if (tr.passed) {
      summary.passed++;
      log(`ok   ${rel} (${durationMs}ms)`);
    } else {
      summary.failed++;
      log(`FAIL ${rel} (${durationMs}ms)`);
      log(`  ${tr.error}`);
    }
  }

  log("");
  log(`${summary.passed} passed, ${summary.failed} failed`);
  return summary;
}
