#!/usr/bin/env node
/**
 * TypeScript Nexus CLI: run / repl / test / selfhost (no Go).
 *
 * Docs: ../docs/README.md  (npm run docs)
 *
 * Usage:
 *   node out/cli.js run <file.nex> [--vm] [--no-serve]
 *   node out/cli.js repl [--vm]
 *   node out/cli.js test [paths...]
 *   node out/cli.js selfhost <file.nex> [-- args...]
 *   node out/cli.js help
 */

import * as fs from "fs";
import * as path from "path";
import { evaluate } from "./compiler/engine";
import { runRepl } from "./repl";
import { runTests } from "./nextest";
import { findStdlibDir, MODULES_DIRNAME } from "./runtime/modules";
import { ArrayObj, IntegerObj, StringObj } from "./language/values";
import { createWebHost, resolveWebHostConfig } from "./host";

async function main(): Promise<void> {
  const args = process.argv.slice(2);
  if (args.length === 0) {
    printUsage();
    process.exit(1);
  }

  const cmd = args[0]!;
  const rest = args.slice(1);

  try {
    switch (cmd) {
      case "run":
        await cmdRun(rest);
        break;
      case "repl":
        await runRepl({
          engine: rest.includes("--vm") ? "vm" : "eval",
          rootDir: process.cwd(),
        });
        break;
      case "test":
        await cmdTest(rest);
        break;
      case "selfhost":
        await cmdSelfhost(rest);
        break;
      case "help":
      case "-h":
      case "--help":
        printUsage();
        break;
      case "version":
      case "-v":
      case "--version":
        // eslint-disable-next-line no-console
        console.log("nex-ts 0.3.0");
        break;
      default:
        // eslint-disable-next-line no-console
        console.error(`unknown command ${JSON.stringify(cmd)}\n`);
        printUsage();
        process.exit(1);
    }
  } catch (err) {
    // eslint-disable-next-line no-console
    console.error(`error: ${err instanceof Error ? err.message : String(err)}`);
    process.exit(1);
  }
}

function printUsage(): void {
  const docsDir = path.resolve(__dirname, "..", "docs");
  // eslint-disable-next-line no-console
  console.error(`nex-ts — Nexus language toolchain (TypeScript)

Usage:
  node out/cli.js <command> [arguments]

Commands:
  run <file.nex> [--vm] [--no-serve]  Execute a Nexus program (TS host; serves HTTP if routes register)
  selfhost <file.nex>                 Run via self-hosted .nex lexer/parser/evaluator
  repl [--vm]                         Interactive read-eval-print loop
  test [paths...]                     Run *_test.nex / tests/**/*.nex
  help                                Show this help
  version                             Show version

Docs:
  ${docsDir}
  Start at docs/README.md  (or: npm run docs)

Examples:
  npm run compile && node out/cli.js run examples/modules_demo.nex
  npm run compile && node out/cli.js selfhost examples/selfhost_demo.nex
  npm run registry
  npm run test:nex
  npm run repl

Registry (HTTP) env:
  NEX_WEB_DIR     Path to web/ (templates + static)
  LISTEN_ADDR     Default :8080
  BASE_URL        Default http://localhost:8080
  STORAGE_DIR     Package artifact directory
`);
}

function findSelfhostMain(rootDir: string): string {
  const candidates = [
    path.join(rootDir, "selfhost", "main.nex"),
    path.join(rootDir, "vscode-nexus", "selfhost", "main.nex"),
    path.join(__dirname, "..", "selfhost", "main.nex"),
  ];
  for (const c of candidates) {
    try {
      if (fs.statSync(c).isFile()) {
        return path.resolve(c);
      }
    } catch {
      // continue
    }
  }
  throw new Error(
    `selfhost/main.nex not found (searched from ${rootDir} and extension)`,
  );
}

async function cmdRun(args: string[]): Promise<void> {
  const useVm = args.includes("--vm");
  const noServe = args.includes("--no-serve");
  const file = args.find((a) => !a.startsWith("-"));
  if (!file) {
    throw new Error("run requires a .nex file");
  }
  const abs = path.resolve(file);
  const source = fs.readFileSync(abs, "utf8");
  const rootDir = process.cwd();

  const hostCfg = resolveWebHostConfig(abs, rootDir);
  if (hostCfg.storageDir) {
    fs.mkdirSync(hostCfg.storageDir, { recursive: true });
  }
  const webHost = createWebHost(hostCfg);

  const result = await evaluate(source, {
    tier: useVm ? "vm" : "eval",
    rootDir: hostCfg.appDir,
    modulesDir: path.join(hostCfg.appDir, MODULES_DIRNAME),
    stdlibDir: findStdlibDir(rootDir),
    filePath: abs,
    checkOwnership: false,
    enableEffects: false,
    webHost,
  });
  for (const line of result.output) {
    // eslint-disable-next-line no-console
    console.log(line);
  }
  for (const d of result.diagnostics) {
    if (d.phase === "vm" || d.phase === "parse") {
      // eslint-disable-next-line no-console
      console.error(`[${d.phase}] ${d.message}`);
    }
  }
  if (result.value.type === "ERROR") {
    // eslint-disable-next-line no-console
    console.error(result.value.inspect());
    process.exit(1);
  }
  if (result.value.type !== "NULL") {
    // eslint-disable-next-line no-console
    console.log(result.value.inspect());
  }

  const forceServe = process.env.NEX_FORCE_SERVE === "1";
  if (!noServe && (webHost.routeCount > 0 || forceServe)) {
    const onSignal = () => {
      void webHost.close().then(() => process.exit(0));
    };
    process.on("SIGINT", onSignal);
    process.on("SIGTERM", onSignal);
    await webHost.listen();
  }
}

/**
 * Bootstrap: TS host loads selfhost/main.nex, which lexes/parses/evals the
 * user program entirely in Nexus. Host still supplies fs_read / puts / __argv__.
 */
async function cmdSelfhost(args: string[]): Promise<void> {
  const dash = args.indexOf("--");
  const fileArgs = dash >= 0 ? args.slice(0, dash) : args;
  const passthrough = dash >= 0 ? args.slice(dash + 1) : [];
  const file = fileArgs.find((a) => !a.startsWith("-"));
  if (!file) {
    throw new Error("selfhost requires a .nex file");
  }

  const rootDir = process.cwd();
  const userFile = path.resolve(file);
  if (!fs.existsSync(userFile)) {
    throw new Error(`file not found: ${userFile}`);
  }

  const mainPath = findSelfhostMain(rootDir);
  const source = fs.readFileSync(mainPath, "utf8");
  const argv = [userFile, ...passthrough].map((s) => new StringObj(s));

  const result = await evaluate(source, {
    tier: "eval",
    rootDir,
    modulesDir: path.join(rootDir, MODULES_DIRNAME),
    stdlibDir: findStdlibDir(rootDir),
    filePath: mainPath,
    checkOwnership: false,
    enableEffects: false,
    bindings: {
      __argv__: new ArrayObj(argv),
    },
  });

  for (const line of result.output) {
    // eslint-disable-next-line no-console
    console.log(line);
  }
  for (const d of result.diagnostics) {
    if (d.phase === "parse" || d.phase === "eval") {
      // eslint-disable-next-line no-console
      console.error(`[${d.phase}] ${d.message}`);
    }
  }
  if (result.value.type === "ERROR") {
    // eslint-disable-next-line no-console
    console.error(result.value.inspect());
    process.exit(1);
  }
  // main() returns 0/1 exit status when successful
  if (result.value instanceof IntegerObj && result.value.value !== 0) {
    process.exit(result.value.value);
  }
}

async function cmdTest(args: string[]): Promise<void> {
  const rootDir = process.cwd();
  const summary = await runTests({
    rootDir,
    paths: args,
    stdlibDir: findStdlibDir(rootDir),
  });
  if (summary.failed > 0) {
    process.exit(1);
  }
}

void main();
