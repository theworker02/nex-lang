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

// Copy static assets — logos must match README (repo assets/logo.*) exactly.
const destImg = path.join(outDir, "static", "img");
const siteImg = path.join(root, "examples", "site", "web", "static", "img");
fs.mkdirSync(destImg, { recursive: true });
fs.mkdirSync(siteImg, { recursive: true });

const assetsDir = path.join(repoRoot, "assets");
const mediaDir = path.join(root, "media");
for (const name of ["logo.svg", "logo.png"]) {
  const canonical = path.join(assetsDir, name);
  const fallback = path.join(mediaDir, name);
  const src = fs.existsSync(canonical)
    ? canonical
    : fs.existsSync(fallback)
      ? fallback
      : null;
  if (!src) continue;
  fs.copyFileSync(src, path.join(destImg, name));
  fs.copyFileSync(src, path.join(siteImg, name));
}

// Keep any other site web assets (non-logo) from examples/site/web
if (fs.existsSync(siteImg)) {
  for (const name of fs.readdirSync(siteImg)) {
    if (name === "logo.svg" || name === "logo.png") continue;
    fs.copyFileSync(path.join(siteImg, name), path.join(destImg, name));
  }
}

const logoPng = path.join(destImg, "logo.png");
const logoSvg = path.join(destImg, "logo.svg");
if (fs.existsSync(logoSvg)) {
  fs.copyFileSync(logoSvg, path.join(destImg, "favicon.svg"));
  fs.copyFileSync(logoSvg, path.join(siteImg, "favicon.svg"));
} else if (fs.existsSync(logoPng)) {
  // favicon.svg preferred; PNG is the visible brand mark (same as README)
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

// Ship CLI zip(s) for direct download on Pages (source: vscode-nexus/downloads/)
const downloadsSrc = path.join(root, "downloads");
const downloadsDest = path.join(outDir, "downloads");
if (fs.existsSync(downloadsSrc)) {
  fs.mkdirSync(downloadsDest, { recursive: true });
  for (const name of fs.readdirSync(downloadsSrc)) {
    if (!name.endsWith(".zip") && !name.endsWith(".txt")) continue;
    fs.copyFileSync(
      path.join(downloadsSrc, name),
      path.join(downloadsDest, name),
    );
    console.log(`  downloads/${name}`);
  }
} else {
  console.warn(
    "warning: vscode-nexus/downloads/ missing — run npm run package:cli",
  );
}

// .nojekyll so GitHub Pages serves paths starting with _
fs.writeFileSync(path.join(outDir, ".nojekyll"), "");
fs.writeFileSync(
  path.join(outDir, "404.html"),
  fs.readFileSync(path.join(outDir, "index.html"), "utf8"),
);

console.log(`static site ready: ${outDir}`);
