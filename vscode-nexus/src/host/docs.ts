/**
 * Built-in documentation pages for the registry (ported subset from Go host).
 */

export interface DocsPage {
  Title: string;
  Lead: string;
  Section: string;
  Body: string;
}

export const DOCS_PAGES: Record<string, DocsPage> = {
  overview: {
    Title: "Overview",
    Section: "guide",
    Lead: "Nexus Registry is the remote package index for the Nexus programming language.",
    Body: `
<h2 id="what">What this server provides</h2>
<p>One process serves both the machine-readable HTTP API and this web UI. Package metadata can live in PostgreSQL (full mode) or an in-memory demo store (TypeScript host without <code>DATABASE_URL</code>). <strong>.nex</strong> package archives are stored on disk and referenced by SHA-256 checksum.</p>
<p>Use the website to search, inspect READMEs, manage API keys, and download releases. Use the API from <code>nex</code>, CI, or any HTTP client.</p>
<h2 id="runtime">TypeScript Nexus runtime</h2>
<p>This deployment is driven by <code>app/*.nex</code> and executed by the TypeScript Nexus CLI (<code>node out/cli.js run</code>). Host builtins provide HTTP routing, templates, static files, and a demo database.</p>
<h2 id="quick-links">Quick links</h2>
<ul>
  <li><a href="/docs/language">Language guide</a></li>
  <li><a href="/docs/runtime">Nexus runtime</a></li>
  <li><a href="/docs/publishing">Publish a package</a></li>
  <li><a href="/docs/api">HTTP API reference</a></li>
  <li><a href="/docs/cli">CLI</a></li>
</ul>
`,
  },
  language: {
    Title: "Language guide",
    Section: "guide",
    Lead: "How Nexus packages relate to the language toolchain and module resolution.",
    Body: `
<h2 id="modules">Packages as modules</h2>
<p>A published Nexus package is a versioned unit of reusable code. The registry stores the artifact and metadata; the CLI resolves versions, verifies checksums, and materializes packages into your project’s cache.</p>
<h2 id="semver">Semantic versioning</h2>
<p>Every release must use semver. Versions are immutable once published — bump the version to ship a change.</p>
<h2 id="manifest">The nexus.toml manifest</h2>
<pre><code>name = "httpkit"
version = "1.0.0"
description = "Tiny HTTP helpers for Nexus"
license = "MIT"
keywords = ["http", "net"]
categories = ["network"]</code></pre>
`,
  },
  installing: {
    Title: "Installing",
    Section: "guide",
    Lead: "Consume .nex packages from the registry with the Nexus CLI or plain HTTP.",
    Body: `
<h2 id="cli-install">With the Nexus CLI</h2>
<pre><code>nex install httpkit@1.0.0</code></pre>
<h2 id="curl-install">With curl</h2>
<pre><code>curl -fsSL http://localhost:8080/api/v1/packages/httpkit/1.0.0/download \\
  -o httpkit-1.0.0.nex</code></pre>
`,
  },
  "getting-started": {
    Title: "Getting started",
    Section: "guide",
    Lead: "Create an account, set up credentials, and publish your first .nex package.",
    Body: `
<h2 id="steps">Steps</h2>
<ol>
  <li>Register at <a href="/register">/register</a>.</li>
  <li>Create an API key under Settings.</li>
  <li>Publish with <code>nex publish</code> (or the HTTP API).</li>
</ol>
`,
  },
  runtime: {
    Title: "Nexus runtime",
    Section: "guide",
    Lead: "The registry is implemented primarily in Nexus (.nex) and executed by the TypeScript Nexus interpreter.",
    Body: `
<h2 id="architecture">Architecture</h2>
<p>Application routes live in <code>app/*.nex</code>. The TypeScript host under <code>vscode-nexus</code> provides HTTP listen/serve, Go-style HTML templates, static files, docs, and an in-memory (or Postgres) data layer.</p>
<pre><code>import "helpers.nex";

http_get("/healthz", fn(req) {
  return json({"status": "ok"});
});</code></pre>
<p>Also supported: <code>if/else</code>, <code>while</code>, <code>return</code>, arrays, maps, imports, and host builtins (<code>http_*</code>, <code>db_*</code>, <code>html</code>, <code>json</code>, <code>config</code>).</p>
`,
  },
  publishing: {
    Title: "Publishing",
    Section: "specs",
    Lead: "Ship immutable .nex releases to the registry.",
    Body: `
<h2 id="publish">Publish</h2>
<pre><code>nex login --token nex_YOUR_KEY
nex publish</code></pre>
<p>Multipart upload posts <code>nexus.toml</code> plus the <code>.nex</code> artifact to <code>POST /api/v1/publish</code>.</p>
`,
  },
  "api-keys": {
    Title: "API keys",
    Section: "specs",
    Lead: "Authenticate CLI and CI uploads with scoped API keys.",
    Body: `<p>Create keys under Settings. Prefer scoped keys for publish-only CI.</p>`,
  },
  "api-keys-deprecation": {
    Title: "API key deprecation",
    Section: "specs",
    Lead: "Rotate and revoke keys safely.",
    Body: `<p>Revoke unused keys promptly. Yanked packages still require owner credentials.</p>`,
  },
  "trusted-publishers": {
    Title: "Trusted publishers",
    Section: "specs",
    Lead: "OIDC-bound CI publishing without long-lived secrets.",
    Body: `<p>Bind a GitHub Actions workflow identity to a package so CI can mint short-lived publish tokens.</p>`,
  },
  "trust-safety": {
    Title: "Trust &amp; safety",
    Section: "specs",
    Lead: "Reporting abuse and malware.",
    Body: `<p>Use <a href="/report">/report</a> or email abuse@nexus-registry.example.</p>`,
  },
  auth: {
    Title: "Authentication",
    Section: "specs",
    Lead: "Sessions, API keys, and OIDC.",
    Body: `<p>Browser sessions use the <code>nex_session</code> cookie. API clients send <code>Authorization: Bearer nex_…</code>.</p>`,
  },
  "nex-files": {
    Title: ".nex files",
    Section: "specs",
    Lead: "The package artifact format.",
    Body: `<p>A <code>.nex</code> file is a gzip-compressed tar archive of package contents plus a <code>nexus.toml</code> manifest.</p>`,
  },
  cli: {
    Title: "CLI reference",
    Section: "commands",
    Lead: "TypeScript Nexus CLI commands for run, test, and registry hosting.",
    Body: `
<pre><code>cd vscode-nexus
npm run compile
node out/cli.js run ../nex-registry/app/main.nex
node out/cli.js selfhost examples/selfhost_demo.nex
npm run test:nex
npm run registry
npm run docs</code></pre>
<p>Language documentation: <code>vscode-nexus/docs/README.md</code>.</p>
`,
  },
  api: {
    Title: "HTTP API",
    Section: "commands",
    Lead: "Machine-readable registry endpoints.",
    Body: `
<pre><code>GET  /healthz
GET  /api/v1/packages
GET  /api/v1/packages/{name}
GET  /api/v1/packages/{name}/{version}
GET  /api/v1/packages/{name}/{version}/download
GET  /api/v1/search?q=
POST /api/v1/publish</code></pre>
`,
  },
};
