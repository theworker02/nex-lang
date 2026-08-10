#!/usr/bin/env node
/**
 * Build a static Nexus language site for GitHub Pages.
 * Uses examples/site design pages (no nex-registry).
 *
 * Env:
 *   NEX_SITE_OUT   — output dir (default: <repo>/site)
 *   NEX_SITE_BASE  — URL prefix for project Pages (e.g. /nex-lang); empty for root
 */

const path = require("path");
const fs = require("fs");
const { spawnSync } = require("child_process");

const root = path.resolve(__dirname, "..");
const repoRoot = path.resolve(root, "..");
const outDir = path.resolve(
  process.env.NEX_SITE_OUT || path.join(repoRoot, "site"),
);
const baseRaw = (process.env.NEX_SITE_BASE || "").trim();
const base = baseRaw.replace(/\/$/, "");

const cli = path.join(root, "out", "cli.js");
if (!fs.existsSync(cli)) {
  console.error("out/cli.js missing — run: npm run compile");
  process.exit(1);
}

fs.rmSync(outDir, { recursive: true, force: true });
fs.mkdirSync(outDir, { recursive: true });

const exportNex = path.join(root, "examples", "site", "export.nex");
const env = {
  ...process.env,
  NEX_SITE_OUT: outDir,
  NEX_APP_DIR: path.join(root, "examples", "site"),
  NEX_WEB_DIR: path.join(root, "examples", "site", "web"),
  NEX_FORCE_SERVE: "0",
};

console.log("Building static Nexus language site");
console.log(`  out=${outDir}`);
console.log(`  base=${base || "(none)"}`);

const result = spawnSync(
  process.execPath,
  [cli, "run", exportNex, "--no-serve"],
  { cwd: root, env, encoding: "utf8" },
);

if (result.stdout) process.stdout.write(result.stdout);
if (result.stderr) process.stderr.write(result.stderr);
if (result.status !== 0) {
  console.error("export.nex failed");
  process.exit(result.status || 1);
}

// Copy static assets (logos)
const srcImg = path.join(root, "examples", "site", "web", "static", "img");
const destImg = path.join(outDir, "static", "img");
fs.mkdirSync(destImg, { recursive: true });
if (fs.existsSync(srcImg)) {
  for (const name of fs.readdirSync(srcImg)) {
    fs.copyFileSync(path.join(srcImg, name), path.join(destImg, name));
  }
} else {
  const media = path.join(root, "media");
  for (const name of ["logo.svg", "logo.png"]) {
    const src = path.join(media, name);
    if (fs.existsSync(src)) {
      fs.copyFileSync(src, path.join(destImg, name));
    }
  }
  const logoSvg = path.join(destImg, "logo.svg");
  if (fs.existsSync(logoSvg)) {
    fs.copyFileSync(logoSvg, path.join(destImg, "favicon.svg"));
  }
}

function applyBase(html) {
  if (!base) return html;
  return html
    .replace(/(href|src|content)="\/(?!\/)/g, `$1="${base}/`)
    .replace(/url\(\//g, `url(${base}/`);
}

function walkHtml(dir) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, ent.name);
    if (ent.isDirectory()) {
      walkHtml(p);
    } else if (ent.name.endsWith(".html")) {
      const html = fs.readFileSync(p, "utf8");
      fs.writeFileSync(p, applyBase(html), "utf8");
    }
  }
}

if (base) {
  walkHtml(outDir);
}

// .nojekyll so GitHub Pages serves paths starting with _
fs.writeFileSync(path.join(outDir, ".nojekyll"), "");
fs.writeFileSync(
  path.join(outDir, "404.html"),
  fs.readFileSync(path.join(outDir, "index.html"), "utf8"),
);

console.log(`static site ready: ${outDir}`);
