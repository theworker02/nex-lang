#!/usr/bin/env node
/**
 * Start the Nexus Registry website via the TypeScript Nexus CLI.
 *
 * Expects sibling checkout: ../nex-registry (or NEX_REGISTRY_DIR).
 */

const path = require("path");
const fs = require("fs");
const { spawn } = require("child_process");

const root = path.resolve(__dirname, "..");

function resolveRegistry() {
  if (process.env.NEX_REGISTRY_DIR) {
    const d = path.resolve(process.env.NEX_REGISTRY_DIR);
    if (fs.existsSync(path.join(d, "app", "main.nex"))) {
      return d;
    }
    throw new Error(`NEX_REGISTRY_DIR set but app/main.nex missing: ${d}`);
  }
  const candidates = [
    // personal projects/nex-lang/vscode-nexus → personal projects/nex-registry
    path.resolve(root, "..", "..", "nex-registry"),
    // nex-lang/vscode-nexus → nex-lang/../nex-registry
    path.resolve(root, "..", "nex-registry"),
    path.resolve(root, "..", "..", "..", "nex-registry"),
  ];
  for (const c of candidates) {
    if (fs.existsSync(path.join(c, "app", "main.nex"))) {
      return c;
    }
  }
  throw new Error(
    `nex-registry not found. Set NEX_REGISTRY_DIR to the registry root.\nTried:\n  ${candidates.join("\n  ")}`,
  );
}

const reg = resolveRegistry();
const mainNex = path.join(reg, "app", "main.nex");
const webDir = path.join(reg, "web");
const storageDir = path.join(reg, "storage");

// Always pin to the resolved registry tree so a relative NEX_WEB_DIR=./web from
// .env does not resolve against vscode-nexus cwd and 500 the site.
process.env.NEX_WEB_DIR = webDir;
process.env.STORAGE_DIR = process.env.STORAGE_DIR && path.isAbsolute(process.env.STORAGE_DIR)
  ? process.env.STORAGE_DIR
  : storageDir;
process.env.LISTEN_ADDR = process.env.LISTEN_ADDR || ":8080";
process.env.BASE_URL = process.env.BASE_URL || "http://localhost:8080";
process.env.NEX_APP_DIR = process.env.NEX_APP_DIR || path.join(reg, "app");
process.env.NEX_REGISTRY_DIR = reg;

const cli = path.join(root, "out", "cli.js");
if (!fs.existsSync(cli)) {
  console.error("out/cli.js missing — run: npm run compile");
  process.exit(1);
}

console.log(`Starting Nexus Registry from ${reg}`);
console.log(`  NEX_WEB_DIR=${process.env.NEX_WEB_DIR}`);
console.log(`  LISTEN_ADDR=${process.env.LISTEN_ADDR}`);
console.log(`  STORAGE_DIR=${process.env.STORAGE_DIR}`);
if (process.env.DATABASE_URL) {
  console.log("  DB mode: Postgres (DATABASE_URL)");
} else {
  console.log("  DB mode: in-memory demo (set DATABASE_URL for Postgres)");
}

const child = spawn(process.execPath, [cli, "run", mainNex], {
  cwd: root,
  env: process.env,
  stdio: "inherit",
});

child.on("exit", (code) => process.exit(code ?? 1));
