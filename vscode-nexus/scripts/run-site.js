#!/usr/bin/env node
/**
 * Serve the standalone Nexus language homepage (examples/site).
 * Self-contained: uses examples/site/web for static assets (logos).
 * Does not require nex-registry.
 */

const path = require("path");
const fs = require("fs");
const { spawn } = require("child_process");

const root = path.resolve(__dirname, "..");
const siteMain = path.join(root, "examples", "site", "main.nex");
const siteWeb = path.join(root, "examples", "site", "web");

function ensureSiteWeb() {
  const imgDir = path.join(siteWeb, "static", "img");
  fs.mkdirSync(imgDir, { recursive: true });
  const media = path.join(root, "media");
  for (const name of ["logo.svg", "logo.png"]) {
    const dest = path.join(imgDir, name);
    if (!fs.existsSync(dest)) {
      const src = path.join(media, name);
      if (fs.existsSync(src)) {
        fs.copyFileSync(src, dest);
      }
    }
  }
  const favicon = path.join(imgDir, "favicon.svg");
  if (!fs.existsSync(favicon)) {
    const logoSvg = path.join(imgDir, "logo.svg");
    if (fs.existsSync(logoSvg)) {
      fs.copyFileSync(logoSvg, favicon);
    }
  }
  return siteWeb;
}

const webDir = ensureSiteWeb();
process.env.LISTEN_ADDR = process.env.LISTEN_ADDR || ":8090";
process.env.BASE_URL = process.env.BASE_URL || "http://localhost:8090";
process.env.NEX_APP_DIR = process.env.NEX_APP_DIR || path.join(root, "examples", "site");
process.env.NEX_FORCE_SERVE = process.env.NEX_FORCE_SERVE || "1";
process.env.NEX_WEB_DIR = webDir;
process.env.STORAGE_DIR =
  process.env.STORAGE_DIR || path.join(root, "examples", "site", "storage");

const cli = path.join(root, "out", "cli.js");
if (!fs.existsSync(cli)) {
  console.error("out/cli.js missing — run: npm run compile");
  process.exit(1);
}

console.log("Starting Nexus language site (standalone)");
console.log(`  main=${siteMain}`);
console.log(`  LISTEN_ADDR=${process.env.LISTEN_ADDR}`);
console.log(`  NEX_WEB_DIR=${process.env.NEX_WEB_DIR}`);

const child = spawn(process.execPath, [cli, "run", siteMain], {
  cwd: root,
  env: process.env,
  stdio: "inherit",
});

child.on("exit", (code) => process.exit(code ?? 1));
