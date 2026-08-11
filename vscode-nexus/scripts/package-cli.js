#!/usr/bin/env node
/**
 * Package the TypeScript Nexus CLI as a downloadable zip for GitHub Releases
 * and the static site (copied into site/downloads/ by build-site.js).
 *
 * Usage (from vscode-nexus):
 *   npm run package:cli
 *
 * Output:
 *   downloads/nex-cli-<version>.zip
 */
const path = require("path");
const fs = require("fs");
const { spawnSync } = require("child_process");

const root = path.resolve(__dirname, "..");
const pkg = JSON.parse(fs.readFileSync(path.join(root, "package.json"), "utf8"));
const version = pkg.version;
const zipName = `nex-cli-${version}.zip`;
const downloadsDir = path.join(root, "downloads");
const stageRoot = path.join(root, ".cli-stage");
const stage = path.join(stageRoot, `nex-cli-${version}`);
const zipPath = path.join(downloadsDir, zipName);

function run(cmd, args, opts = {}) {
  const r = spawnSync(cmd, args, {
    stdio: "inherit",
    shell: process.platform === "win32",
    ...opts,
  });
  if (r.status !== 0) {
    process.exit(r.status || 1);
  }
}

function rmrf(p) {
  fs.rmSync(p, { recursive: true, force: true });
}

function copyJsTree(src, dest) {
  fs.mkdirSync(dest, { recursive: true });
  for (const ent of fs.readdirSync(src, { withFileTypes: true })) {
    const from = path.join(src, ent.name);
    const to = path.join(dest, ent.name);
    if (ent.isDirectory()) {
      copyJsTree(from, to);
    } else if (ent.name.endsWith(".js")) {
      fs.copyFileSync(from, to);
    }
  }
}

function copyDir(src, dest) {
  fs.mkdirSync(dest, { recursive: true });
  for (const ent of fs.readdirSync(src, { withFileTypes: true })) {
    const from = path.join(src, ent.name);
    const to = path.join(dest, ent.name);
    if (ent.isDirectory()) {
      copyDir(from, to);
    } else {
      fs.copyFileSync(from, to);
    }
  }
}

const outCli = path.join(root, "out", "cli.js");
if (!fs.existsSync(outCli)) {
  console.log("compiling…");
  run("npm", ["run", "compile"], { cwd: root });
}

rmrf(stageRoot);
fs.mkdirSync(stage, { recursive: true });
fs.mkdirSync(downloadsDir, { recursive: true });

const cliPkg = {
  name: "nex-cli",
  version,
  description:
    "Nexus language CLI (TypeScript toolchain) — run / repl / test / selfhost",
  license: "MIT",
  private: true,
  engines: { node: ">=18" },
  dependencies: {
    axios: pkg.dependencies.axios,
    "form-data": pkg.dependencies["form-data"],
    pg: pkg.dependencies.pg,
    "smol-toml": pkg.dependencies["smol-toml"],
    tar: pkg.dependencies.tar,
  },
  optionalDependencies: pkg.optionalDependencies || {},
};
fs.writeFileSync(
  path.join(stage, "package.json"),
  JSON.stringify(cliPkg, null, 2) + "\n",
  "utf8",
);

const readme = `Nexus CLI (nex-cli) v${version}
==========================

TypeScript Nexus toolchain: run, REPL, tests, and selfhost.

Requirements
------------
- Node.js 18+ on your PATH

Quick start
-----------
1. Unzip this archive.
2. Open a terminal in the nex-cli-${version} folder.
3. Dependencies are already included under node_modules/.
   If you need to reinstall: npm install --omit=dev
4. Try:

   node out/cli.js help
   node out/cli.js version
   node out/cli.js run path/to/program.nex
   node out/cli.js run path/to/program.nex --vm
   node out/cli.js repl
   node out/cli.js test
   node out/cli.js selfhost path/to/program.nex

Notes
-----
- stdlib/ and selfhost/ ship with this package.
- Also available on GitHub Releases and this project's Pages downloads/.
- VS Code / VSCodium: see the nex-lsp VSIX on the same release.

Repo: https://github.com/theworker02/nex-lang
Release: https://github.com/theworker02/nex-lang/releases/tag/v${version}
Site download: https://theworker02.github.io/nex-lang/downloads/${zipName}
`;
fs.writeFileSync(path.join(stage, "README.txt"), readme, "utf8");

copyJsTree(path.join(root, "out"), path.join(stage, "out"));
copyDir(path.join(root, "stdlib"), path.join(stage, "stdlib"));
copyDir(path.join(root, "selfhost"), path.join(stage, "selfhost"));

console.log("npm install --omit=dev (staging)…");
run("npm", ["install", "--omit=dev", "--no-fund", "--no-audit"], {
  cwd: stage,
});

if (fs.existsSync(zipPath)) fs.unlinkSync(zipPath);

console.log(`zip → ${zipPath}`);
if (process.platform === "win32") {
  // Compress-Archive expects the folder path; nests as nex-cli-VERSION/...
  run("powershell", [
    "-NoProfile",
    "-Command",
    `Compress-Archive -Path '${stage.replace(/'/g, "''")}' -DestinationPath '${zipPath.replace(/'/g, "''")}' -CompressionLevel Optimal`,
  ]);
} else {
  run("zip", ["-r", "-q", zipPath, `nex-cli-${version}`], {
    cwd: stageRoot,
    shell: false,
  });
}

rmrf(stageRoot);

const bytes = fs.statSync(zipPath).size;
console.log(`CLI package ready: ${zipPath} (${Math.round(bytes / 1024)} KiB)`);
