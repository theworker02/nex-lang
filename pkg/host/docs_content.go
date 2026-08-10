package host

import "html/template"

type docsPage struct {
	Title   string
	Lead    string
	Section string // sidebar accordion section id
	Body    template.HTML
}

var docsPages = map[string]docsPage{
	"overview": {
		Title:   "Overview",
		Section: "guide",
		Lead:    "Nexus Registry is the remote package index for the Nexus programming language.",
		Body: template.HTML(`
<h2 id="what">What this server provides</h2>
<p>One process serves both the machine-readable HTTP API and this web UI. Package metadata lives in PostgreSQL; <strong>.nex</strong> package archives are stored on disk and referenced by SHA-256 checksum.</p>
<p>Use the website to search, inspect READMEs, manage API keys, and download releases. Use the API from <code>nex</code>, CI, or any HTTP client.</p>
<h2 id="nex-files-intro">What is a .nex file?</h2>
<p>A <code>.nex</code> file is the first-class Nexus package artifact — a gzip-compressed tar archive of the package contents (same idea as crates.io’s <code>.crate</code> or npm’s <code>.tgz</code>). Every published version stores one immutable <code>.nex</code> file plus a <code>nexus.toml</code> manifest.</p>
<h2 id="accounts">Accounts &amp; publishing</h2>
<p>Publishing requires an account. After registering you can generate an API key, optionally opt into Gravatar for your avatar, configure trusted publishers for CI, and publish your first package. See the <a href="/docs/getting-started">Getting started</a> guide.</p>
<h2 id="quick-links">Quick links</h2>
<ul>
  <li><a href="/docs/language">Language guide</a></li>
  <li><a href="/docs/nex-files">.nex package format</a></li>
  <li><a href="/docs/api-keys">API keys</a></li>
  <li><a href="/docs/trusted-publishers">Trusted publishers</a></li>
  <li><a href="/docs/publishing">Publish a package</a></li>
  <li><a href="/docs/api">HTTP API reference</a></li>
</ul>
`),
	},
	"language": {
		Title:   "Language guide",
		Section: "guide",
		Lead:    "How Nexus packages relate to the language toolchain and module resolution.",
		Body: template.HTML(`
<h2 id="modules">Packages as modules</h2>
<p>A published Nexus package is a versioned unit of reusable code. The registry stores the artifact and metadata; the <code>nex</code> CLI resolves versions, verifies checksums, and materializes packages into your project’s cache.</p>
<h2 id="semver">Semantic versioning</h2>
<p>Every release must use semver (<code>MAJOR.MINOR.PATCH</code> with optional pre-release / build metadata). Versions are immutable once published — bump the version to ship a change.</p>
<h2 id="manifest">The nexus.toml manifest</h2>
<p>Package identity and discoverability fields live in <code>nexus.toml</code>:</p>
<pre><code>name = "httpkit"
version = "1.2.0"
description = "Tiny HTTP helpers for Nexus"
license = "MIT"
keywords = ["http", "net"]
categories = ["network"]

[dependencies]
jsonutil = "1.0.0"</code></pre>
<h2 id="deps">Dependencies</h2>
<p>Dependencies declared under <code>[dependencies]</code> are recorded on the published version and shown on the package page as a dependency list. Resolution rules are enforced by the Nexus CLI; the registry stores the declared requirements as published.</p>
`),
	},
	"installing": {
		Title:   "Installing",
		Section: "guide",
		Lead:    "Consume .nex packages from the registry with the Nexus CLI or plain HTTP.",
		Body: template.HTML(`
<h2 id="cli-install">With the Nexus CLI</h2>
<pre><code>nex install httpkit@1.2.0</code></pre>
<p>The CLI fetches metadata from <code>/api/v1/packages/{name}/{version}</code>, downloads the <code>.nex</code> archive, verifies the <code>sha256:</code> checksum, and installs into the local cache.</p>
<h2 id="curl-install">With curl</h2>
<pre><code>curl -fsSL http://localhost:8080/api/v1/packages/httpkit/1.2.0/download \
  -o httpkit-1.2.0.nex
# verify the sha256: checksum from the package JSON before installing</code></pre>
<h2 id="browse">Browse before you install</h2>
<p>Package pages expose install commands, checksums, version history, dependency lists, and README content rendered from Markdown.</p>
`),
	},
	"getting-started": {
		Title:   "Getting started",
		Section: "guide",
		Lead:    "Create an account, set up credentials, and publish your first .nex package.",
		Body: template.HTML(`
<h2 id="checklist">Checklist</h2>
<ol>
  <li><a href="/register">Create an account</a> (optional: enable Gravatar for your avatar).</li>
  <li>Open <a href="/settings">Account settings</a> and <strong>generate an API key</strong>. Copy it immediately — it is shown only once.</li>
  <li>Authenticate the CLI: <code>nex login --token nex_…</code> (or <code>nex login --browser</code>). See <a href="/docs/cli">CLI reference</a>.</li>
  <li>(Optional) Configure a <a href="/docs/trusted-publishers">trusted publisher</a> for GitHub Actions.</li>
  <li>Publish with <code>nex publish</code> (see <a href="/docs/publishing">Publishing</a>).</li>
</ol>
<h2 id="after-signup">After signup</h2>
<p>New accounts are redirected to the interactive <a href="/getting-started">Getting started</a> checklist, which tracks whether you have API keys, trusted publishers, and published packages.</p>
`),
	},
	"nex-files": {
		Title:   ".nex files",
		Section: "specs",
		Lead:    "The Nexus package archive format registered by this registry.",
		Body: template.HTML(`
<h2 id="format">Format</h2>
<p>A <code>.nex</code> package is a <strong>gzip-compressed tar</strong> archive with the <code>.nex</code> extension. Content type on download is <code>application/x-nexus-package</code>.</p>
<p>Recommended layout inside the archive:</p>
<pre><code>package-root/
  nexus.toml
  src/
  README.md
  …</code></pre>
<h2 id="immutability">Immutability</h2>
<p>Once a <code>name</code> + <code>version</code> is published, the stored <code>.nex</code> file and its checksum never change. Republishing the same version returns HTTP 409.</p>
<h2 id="checksums">Checksums</h2>
<p>The registry records <code>sha256:&lt;hex&gt;</code> for every upload. Clients must verify the downloaded bytes before install.</p>
<h2 id="legacy">Legacy uploads</h2>
<p>Publish still accepts <code>.tar.gz</code> / <code>.tgz</code> for migration, but the registry always stores the artifact as <code>{name}-{version}.nex</code>.</p>
`),
	},
	"publishing": {
		Title:   "Publishing",
		Section: "specs",
		Lead:    "Register a new immutable package version with nexus.toml, a .nex archive, and an API key.",
		Body: template.HTML(`
<h2 id="auth-required">Authentication required</h2>
<p>Publish requires a logged-in session <em>or</em> an API key:</p>
<pre><code>Authorization: Bearer nex_…
# or
X-Api-Key: nex_…</code></pre>
<p>Generate keys in <a href="/settings">Account settings</a>. See <a href="/docs/api-keys">API keys</a>.</p>
<h2 id="manifest">Manifest</h2>
<pre><code>name = "httpkit"
version = "1.2.0"
description = "Tiny HTTP helpers for Nexus"
author = "you@example.com"
license = "MIT"
repository = "https://github.com/example/httpkit"
homepage = "https://example.com/httpkit"
keywords = ["http", "net", "client"]
categories = ["network"]

[dependencies]
jsonutil = "1.0.0"</code></pre>
<h2 id="upload">Upload contract</h2>
<ul>
  <li><code>nexus.toml</code> — required manifest</li>
  <li><code>package</code> — required <code>.nex</code> file (preferred field name)</li>
  <li><code>archive</code> — accepted alias for <code>package</code></li>
  <li><code>readme</code> — optional markdown body</li>
</ul>
<pre><code># Preferred: Nexus CLI
nex login --token nex_YOUR_KEY
nex publish

# Or curl
curl -X POST http://localhost:8080/api/v1/publish \
  -H "Authorization: Bearer nex_YOUR_KEY" \
  -F "nexus.toml=@nexus.toml" \
  -F "package=@httpkit-1.2.0.nex" \
  -F "readme=@README.md"</code></pre>
<p>The first publish sets you as the package owner. Later versions must come from the same owner. See <a href="/docs/cli">CLI reference</a> for install/yank.</p>
<h2 id="rate-limit">Publish cooldown</h2>
<p>To prevent spam uploads, each account may complete <strong>one successful publish</strong> every <strong>30 minutes</strong> by default (configurable via <code>PUBLISH_RATE_LIMIT_MINUTES</code>, maximum 60). The limit applies to the authenticated user (API key owner, session, or trusted-publisher identity), not only the client IP.</p>
<p>A second publish inside the cooldown window returns <strong>HTTP 429</strong> with <code>Retry-After</code> and a JSON <code>error</code> describing how long to wait. Failed validation (4xx other than 429) does not start a new cooldown. Details: <a href="/docs/trust-safety#rate-limits">Trust &amp; safety → Rate limits</a>.</p>
`),
	},
	"api-keys": {
		Title:   "API keys",
		Section: "specs",
		Lead:    "Scoped long-lived credentials for publishing and read automation (migrate to Trusted Publishers by 2027).",
		Body: template.HTML(`
<h2 id="create">Create a key</h2>
<ol>
  <li>Sign in and open <a href="/settings">Account settings</a>.</li>
  <li>Under <strong>API keys</strong>, choose a label, <strong>scope</strong>, and optional expiry.</li>
  <li>Copy the full <code>nex_…</code> secret immediately. It is shown <strong>only once</strong>.</li>
</ol>
<p>The registry stores only a SHA-256 hash of the key plus a short prefix for identification (for example <code>nex_a1b2c3d4…</code>).</p>
<h2 id="scopes">Scopes</h2>
<ul>
  <li><code>publish</code> — publish packages (default).</li>
  <li><code>read</code> — authenticated read-only API access; cannot publish.</li>
  <li><code>full</code> — publish + read.</li>
</ul>
<p>Keys may set <code>expires_days</code> (or the settings “Expires in days” field). Expired keys are rejected.</p>
<h2 id="migrate-2027">Migration toward Trusted Publishers (2027)</h2>
<p>Publishing with long-lived API keys will stop after <strong>February 1st, 2027</strong>. Move CI to <a href="/docs/trusted-publishers">Trusted Publishers</a> (GitHub Actions OIDC). Keep short-lived or read-only keys only where needed. Full timeline: <a href="/docs/api-keys-deprecation">API key deprecation</a>.</p>
<h2 id="use">Use with curl / nex</h2>
<pre><code>curl -X POST http://localhost:8080/api/v1/publish \
  -H "Authorization: Bearer nex_YOUR_KEY" \
  -F "nexus.toml=@nexus.toml" \
  -F "package=@pkg-1.0.0.nex"</code></pre>
<p>You may also send <code>X-Api-Key: nex_YOUR_KEY</code>.</p>
<h2 id="api">JSON API</h2>
<pre><code>POST   /api/user/api-keys   # body: { "name", "scope", "expires_days" }
GET    /api/user/api-keys
DELETE /api/user/api-keys/{id}</code></pre>
<h2 id="revoke">Revoke</h2>
<p>Revoke compromised keys from settings or via <code>DELETE /api/user/api-keys/{id}</code>. Revoked keys stop working immediately. Creates/revokes are written to the audit log.</p>
`),
	},
	"trust-safety": {
		Title:   "Trust & safety",
		Section: "specs",
		Lead:    "2FA, yank rules, rate limits, audit logs, provenance hooks, and abuse reporting.",
		Body: template.HTML(`
<h2 id="2fa">Two-factor authentication (TOTP)</h2>
<p>Enable an authenticator under <a href="/settings#security">Settings → Two-factor authentication</a>. After password login you will be challenged at <code>/login/2fa</code>. JSON clients use <code>POST /api/auth/login</code> → <code>requires_2fa</code> + <code>challenge</code>, then <code>POST /api/auth/2fa</code>.</p>
<h2 id="yank">Yank / unpublish</h2>
<ul>
  <li><strong>Yank</strong> — owner marks a version yanked with a reason. Metadata remains; default downloads return <code>410</code> unless <code>allow_yanked=1</code> (for existing lockfiles). Install UIs warn.</li>
  <li><strong>Unyank</strong> — clears the yanked flag.</li>
  <li><strong>Unpublish</strong> — hard-delete within <strong>72 hours</strong> of publish via <code>POST /api/v1/packages/{name}/{version}/unpublish</code>. After that, yank instead.</li>
</ul>
<pre><code>POST /api/v1/packages/{name}/{version}/yank     # { "reason": "…" }
POST /api/v1/packages/{name}/{version}/unyank
POST /api/v1/packages/{name}/{version}/unpublish</code></pre>
<h2 id="rate-limits">Rate limits</h2>
<p><strong>Publish cooldown (per account):</strong> after a successful publish, the same authenticated identity (session user, API key owner, or trusted-publisher user) must wait before publishing again. Default cooldown is <strong>30 minutes</strong> (1 successful publish per window). Configure with <code>PUBLISH_RATE_LIMIT_MINUTES</code> (integer 1–60). Cooldowns are stored in Postgres (<code>publish_rate_limits</code>) so restarts do not reset the window.</p>
<p>When limited, <code>POST /api/v1/publish</code> returns HTTP <strong>429</strong> with a <code>Retry-After</code> header (seconds) and JSON such as:</p>
<pre><code>{
  "error": "publish rate limit: wait 28 minute(s) before publishing again (cooldown 30 minutes between successful publishes)",
  "retry_after_seconds": 1680,
  "cooldown_minutes": 30
}</code></pre>
<p><strong>Secondary IP limits</strong> (token bucket): auth ~20/min, publish ~8/hour per IP, search ~60/min. Exceeding also returns HTTP 429.</p>
<p>See also <a href="/docs/publishing#rate-limit">Publishing → Rate limit</a>.</p>
<h2 id="audit">Audit logs</h2>
<p>Publish, API key, trusted-publisher, yank, and 2FA events are recorded. See <a href="/settings#audit">Settings → Recent security activity</a> or <code>GET /api/user/audit-logs</code>.</p>
<h2 id="provenance">Provenance hook</h2>
<p>On publish you may attach optional Sigstore/OIDC provenance JSON via multipart field <code>provenance</code> (+ <code>provenance_source</code>) or headers <code>X-Nexus-Provenance</code> / <code>X-Nexus-Provenance-Source</code>. GitHub OIDC publishes store a minimal OIDC claim snapshot automatically. Full Sigstore cryptographic verification is stubbed behind a clear <code>ProvenanceVerifier</code> interface in the host.</p>
<h2 id="abuse">Report abuse</h2>
<p>Use <a href="/report">/report</a> or <code>POST /api/v1/report</code>. Admins review at <a href="/admin/abuse">/admin/abuse</a> (requires <code>users.is_admin</code>).</p>
`),
	},
	"api-keys-deprecation": {
		Title:   "API key deprecation",
		Section: "specs",
		Lead:    "API keys are being replaced by Trusted Publishers (GitHub Actions OIDC).",
		Body: template.HTML(`
<h2 id="deadline">What is changing</h2>
<p>Long-lived <code>nex_…</code> API keys will be <strong>discontinued and no longer working after February 1st, 2027 at 12:00 AM</strong>.</p>
<p>After that date, publishing from automation must use <a href="/docs/trusted-publishers">Trusted Publishers</a> — GitHub Actions OIDC identities bound to your registry account — instead of copying secrets into CI.</p>
<h2 id="why">Why</h2>
<ul>
  <li>API keys are long-lived bearer secrets that can leak from logs, forks, or compromised runners.</li>
  <li>Trusted publishing verifies a short-lived JWT from GitHub (<code>iss=https://token.actions.githubusercontent.com</code>) against your configured repository, workflow, and optional environment.</li>
  <li>This matches the direction taken by PyPI and npm.</li>
</ul>
<h2 id="migrate">How to migrate</h2>
<ol>
  <li>Open <a href="/settings#trusted-publishers">Account settings → Trusted publishers</a>.</li>
  <li>Add your GitHub <code>owner</code> / <code>repository</code> (pin workflow filename and environment when you can).</li>
  <li>Add the <a href="/docs/trusted-publishers#github-actions-yml">publish workflow YAML</a> to that repository.</li>
  <li>Revoke unused API keys when CI is green on OIDC.</li>
</ol>
<h2 id="timeline">Timeline</h2>
<ul>
  <li><strong>Now</strong> — API keys and Trusted Publishers both work.</li>
  <li><strong>February 1st, 2027, 12:00 AM</strong> — API keys stop authenticating publish requests.</li>
</ul>
<p>Configure Trusted Publishers today: <a href="/settings#trusted-publishers">Settings</a> · <a href="/docs/trusted-publishers">Full guide</a>.</p>
`),
	},
	"trusted-publishers": {
		Title:   "Trusted publishers",
		Section: "specs",
		Lead:    "RubyGems-style pending publishers: claim a package name before first publish, then verify via GitHub Actions OIDC.",
		Body: template.HTML(`
<h2 id="concept">Concept</h2>
<p>Trusted publishers declare which GitHub Actions workflows may publish on your behalf (RubyGems / PyPI style). The registry verifies the Actions OIDC JWT, matches it to your configuration, and allows <code>POST /api/v1/publish</code> without an API key.</p>
<p>This is the supported publish path after <a href="/docs/api-keys-deprecation">API key deprecation</a> (February 1st, 2027).</p>

<h2 id="pending">Pending → verified lifecycle</h2>
<ol>
  <li><strong>Pending</strong> — register under <a href="/settings#trusted-publishers">Settings</a> <em>before</em> the package exists. You must enter GitHub owner, repository, <strong>exact workflow file path</strong>, package name to claim, and optional environment.</li>
  <li>Create the workflow file in that repository with the <strong>same path</strong> you entered (for example <code>.github/workflows/release.yml</code>).</li>
  <li>Run the workflow. The first successful OIDC publish of the claimed package name marks the publisher <strong>verified</strong> (active).</li>
  <li>Mismatches (wrong workflow file, repo, environment, or package) return <code>403</code> and store a <em>last failure</em> reason on the pending row in Settings.</li>
</ol>
<p>You may configure <strong>multiple</strong> pending and verified publishers per user/package (different repos or workflows).</p>

<h2 id="fields">Configuration fields</h2>
<ul>
  <li><strong>Provider</strong> — <code>github_actions</code></li>
  <li><strong>Repository owner / name</strong> — e.g. <code>acme</code> / <code>httpkit</code> (required)</li>
  <li><strong>Workflow filename</strong> — required exact path, e.g. <code>.github/workflows/release.yml</code> (basename match also accepted)</li>
  <li><strong>Package name to claim</strong> — required for pending publishers; must match <code>nexus.toml</code> <code>name</code> on first publish</li>
  <li><strong>Environment</strong> — optional GitHub Environment name (e.g. <code>release</code>)</li>
  <li><strong>Status</strong> — <code>pending</code> until first successful OIDC publish, then <code>verified</code></li>
</ul>
<p>Manage configs under <a href="/settings#trusted-publishers">Account settings</a>.</p>

<h2 id="oidc-flow">OIDC publish flow</h2>
<ol>
  <li>GitHub Actions requests an OIDC token with <code>permissions: id-token: write</code> and audience <code>nex-registry</code> (or your registry base URL).</li>
  <li>Optional: exchange the JWT for a short-lived <code>nxt_…</code> token via <code>POST /api/v1/trusted-publishing/token</code>.</li>
  <li>Upload <code>nexus.toml</code> + the <code>.nex</code> artifact to <code>POST /api/v1/publish</code> with <code>Authorization: Bearer &lt;jwt or nxt_…&gt;</code>.</li>
  <li>The registry verifies issuer <code>https://token.actions.githubusercontent.com</code>, signature (JWKS), audience, expiry, then matches <code>repository</code>, workflow, environment, and package name. A matching <strong>pending</strong> publisher may authorize the first publish of that package.</li>
</ol>
<h2 id="endpoints">HTTP endpoints</h2>
<pre><code>POST /api/v1/trusted-publishing/token   # Bearer: GitHub OIDC JWT → { token: "nxt_…", expires_in }
POST /api/v1/publish                    # Bearer: GitHub OIDC JWT or nxt_… (multipart nexus.toml + package)</code></pre>
<h2 id="github-actions-yml">GitHub Actions YAML</h2>
<p>Create the file at the path you registered (example uses <code>.github/workflows/release.yml</code> — change both Settings and the path if you prefer another name):</p>
<pre><code>name: Publish to Nexus Registry

on:
  push:
    tags: ["v*"]
  workflow_dispatch:

permissions:
  id-token: write   # required for OIDC
  contents: read

jobs:
  publish:
    runs-on: ubuntu-latest
    # environment: release   # optional; must match Settings if set
    steps:
      - uses: actions/checkout@v4

      - name: Package
        run: |
          VERSION="${GITHUB_REF_NAME#v}"
          NAME=$(grep -E '^name\s*=' nexus.toml | head -1 | cut -d'"' -f2)
          tar -czf "${NAME}-${VERSION}.nex" \
            --exclude .git --exclude .github --exclude "*.nex" .
          echo "NAME=$NAME" &gt;&gt; "$GITHUB_ENV"
          echo "VERSION=$VERSION" &gt;&gt; "$GITHUB_ENV"

      - name: Request OIDC token
        id: oidc
        run: |
          TOKEN=$(curl -sS -H "Authorization: Bearer $ACTIONS_ID_TOKEN_REQUEST_TOKEN" \
            "${ACTIONS_ID_TOKEN_REQUEST_URL}&amp;audience=nex-registry" | jq -r .value)
          echo "::add-mask::$TOKEN"
          echo "token=$TOKEN" &gt;&gt; "$GITHUB_OUTPUT"

      - name: Exchange for publish token
        id: mint
        env:
          REGISTRY: https://registry.example.com   # or http://localhost:8080
        run: |
          RESP=$(curl -sS -X POST "$REGISTRY/api/v1/trusted-publishing/token" \
            -H "Authorization: Bearer ${{ steps.oidc.outputs.token }}" \
            -H "Accept: application/json")
          PUB=$(echo "$RESP" | jq -r .token)
          echo "::add-mask::$PUB"
          echo "token=$PUB" &gt;&gt; "$GITHUB_OUTPUT"

      - name: Publish
        env:
          REGISTRY: https://registry.example.com
        run: |
          curl -sS -X POST "$REGISTRY/api/v1/publish" \
            -H "Authorization: Bearer ${{ steps.mint.outputs.token }}" \
            -F "nexus.toml=@nexus.toml" \
            -F "package=@${NAME}-${VERSION}.nex"</code></pre>
<p>You may also skip the exchange step and call <code>/api/v1/publish</code> directly with the GitHub OIDC JWT as the Bearer token.</p>
<h2 id="audience">OIDC audience</h2>
<p>Accepted audiences: your registry <code>BASE_URL</code> (e.g. <code>http://localhost:8080</code>), <code>nex-registry</code>, and any value in <code>NEX_OIDC_AUDIENCE</code>.</p>
<h2 id="failures">Failure reasons</h2>
<p>When OIDC identity does not match a pending/verified config, Settings shows the last failure (wrong workflow file, environment, package name, or missing repo config). HTTP responses include the same message as <code>403</code>.</p>
<h2 id="crud-api">JSON API (account)</h2>
<pre><code>GET    /api/user/trusted-publishers
POST   /api/user/trusted-publishers   # requires workflow_filename + package_scope; creates status=pending
DELETE /api/user/trusted-publishers/{id}</code></pre>
`),
	},
	"auth": {
		Title:   "Authentication",
		Section: "specs",
		Lead:    "Session cookies for the website and Bearer tokens / API keys for the HTTP API.",
		Body: template.HTML(`
<h2 id="sessions">Website sessions</h2>
<p>Register and log in through the web UI. The server sets an HttpOnly <code>nex_session</code> cookie. Session tokens are stored hashed in PostgreSQL and expire after 30 days.</p>
<h2 id="github-oauth">Sign in with GitHub</h2>
<p>Optional GitHub OAuth is available when the server is configured with a GitHub OAuth App:</p>
<pre><code>GITHUB_CLIENT_ID=…
GITHUB_CLIENT_SECRET=…
GITHUB_REDIRECT_URI=http://localhost:8080/auth/github/callback   # optional; defaults to BASE_URL + /auth/github/callback
BASE_URL=http://localhost:8080</code></pre>
<ol>
  <li>Create an OAuth App under GitHub → Settings → Developer settings → OAuth Apps.</li>
  <li>Set Authorization callback URL to <code>/auth/github/callback</code> on your registry host.</li>
  <li>Restart the registry with the env vars above.</li>
</ol>
<p>Routes: <code>GET /auth/github</code> (start) and <code>GET /auth/github/callback</code>. First login creates a registry user from the GitHub login/avatar; existing users can link GitHub from Settings. Email/password login remains available.</p>
<h2 id="api-auth">API authentication</h2>
<ul>
  <li><code>Authorization: Bearer nxs_…</code> — session token from login/register JSON</li>
  <li><code>Authorization: Bearer nex_…</code> — API key</li>
  <li><code>X-Api-Key: nex_…</code> — API key</li>
</ul>
<pre><code>POST /api/auth/register
POST /api/auth/login
POST /api/auth/logout
GET  /api/user/profile</code></pre>
<h2 id="gravatar">Gravatar</h2>
<p>During registration or in settings you can opt into Gravatar. The registry computes <code>SHA-256</code> of your normalized email and sets <code>avatar_url</code> to the Gravatar CDN. You can disable Gravatar later and set a custom avatar URL.</p>
`),
	},
	"cli": {
		Title:   "CLI reference",
		Section: "commands",
		Lead:    "The nex CLI talks to this registry like cargo/npm — login, publish, install, yank.",
		Body: template.HTML(`
<h2 id="build">Build the CLI</h2>
<p>From the sibling <code>nex-lang</code> repository:</p>
<pre><code>go build -o bin/nex.exe ./cmd/nex
# optional: add bin/ to PATH
</code></pre>
<h2 id="config">Registry URL &amp; credentials</h2>
<p>Config file: <code>$CONFIG/nex/config.toml</code> (on Windows, under <code>%AppData%\nex\config.toml</code>).</p>
<pre><code>registry_url = "http://localhost:8080"
token = "nex_…"
username = "you"</code></pre>
<ul>
  <li><code>NEX_REGISTRY_URL</code> — overrides <code>registry_url</code></li>
  <li><code>NEX_TOKEN</code> or <code>NEX_API_KEY</code> — overrides stored token (API key <code>nex_…</code> or session <code>nxs_…</code>)</li>
</ul>
<h2 id="login-cmd">nex login</h2>
<pre><code># Paste an API key from Account settings
nex login --token nex_YOUR_KEY

# Open settings in a browser, then paste a key
nex login --browser

# Username/password → session token (add --api-key to mint a long-lived key instead)
nex login --username you
nex login --api-key

nex logout</code></pre>
<h2 id="publish-cmd">nex publish</h2>
<pre><code>nex publish</code></pre>
<p>Bundles the current directory as a <code>.nex</code> archive and uploads multipart <code>nexus.toml</code> + <code>package</code> to <code>POST /api/v1/publish</code> (auth required).</p>
<h2 id="install-cmd">nex install</h2>
<pre><code>nex install httpkit@1.2.0
nex install httpkit          # newest non-yanked version</code></pre>
<p>Fetches <code>/api/v1/packages/{name}/{version}</code>, downloads the artifact, verifies the <code>sha256:</code> checksum, and extracts into <code>.modules/&lt;name&gt;/</code>.</p>
<h2 id="yank-cmd">nex yank</h2>
<pre><code>nex yank httpkit@1.2.0 --reason "critical bug in helper"</code></pre>
<p>Calls <code>POST /api/v1/packages/{name}/{version}/yank</code> with a required reason. Yanked versions stay downloadable with <code>allow_yanked=1</code> for lockfiles, but are skipped by default install resolution.</p>
<h2 id="search-cmd">Search</h2>
<pre><code>curl -s "http://localhost:8080/api/v1/search?q=http&amp;category=network&amp;sort=relevance" | jq
curl -s "http://localhost:8080/api/v1/search?keyword=async&amp;license=MIT&amp;updated_after=30d&amp;sort=downloads" | jq</code></pre>
`),
	},
	"api": {
		Title:   "HTTP API",
		Section: "commands",
		Lead:    "Stable JSON endpoints for tooling and the Nexus CLI.",
		Body: template.HTML(`
<h2 id="registry">Registry</h2>
<pre><code>POST /api/v1/publish                          # auth required
POST /api/v1/packages/{name}/{version}/yank   # auth + owner; body { "reason": "…" }
POST /api/v1/packages/{name}/{version}/unyank # auth + owner
GET  /api/v1/packages?page=1&amp;per_page=25
GET  /api/v1/packages/{name}
GET  /api/v1/packages/{name}/{version}
GET  /api/v1/packages/{name}/{version}/download  # ?allow_yanked=1 for yanked
GET  /api/v1/search?q=&amp;category=&amp;keyword=&amp;license=&amp;updated_after=&amp;sort=
GET  /api/v1/keywords?limit=36
GET  /api/v1/licenses?limit=24
GET  /api/v1/users/{username}
GET  /healthz</code></pre>
<p>Search ranks exact/prefix name matches above keyword hits, then description/full-text. Downloads and recent updates add a boost. <code>sort</code> may be <code>relevance</code>, <code>downloads</code>, or <code>recent</code>. <code>updated_after</code> accepts <code>YYYY-MM-DD</code>, RFC3339, or relative windows like <code>7d</code>/<code>30d</code>/<code>90d</code>.</p>
<pre><code>GET /search
GET /packages?q=&amp;category=&amp;keyword=&amp;license=&amp;updated_after=&amp;sort=
GET /keywords/{keyword}
GET /categories/{slug}
GET /discover</code></pre>
<h2 id="auth-api">Auth &amp; account</h2>
<pre><code>POST   /api/auth/register
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/user/profile
PATCH  /api/user/profile
GET    /api/user/api-keys
POST   /api/user/api-keys
DELETE /api/user/api-keys/{id}
GET    /api/user/trusted-publishers
POST   /api/user/trusted-publishers
DELETE /api/user/trusted-publishers/{id}</code></pre>
<h2 id="example">Example: resolve a package</h2>
<pre><code>curl -s http://localhost:8080/api/v1/packages/httpkit/1.2.0 | jq</code></pre>
`),
	},
	"runtime": {
		Title:   "Nexus runtime",
		Section: "guide",
		Lead:    "This registry is implemented primarily in Nexus (.nex) and executed by the nex-lang interpreter — with gradual types, pattern matching, Result/try, structs, and pipes.",
		Body: template.HTML(`
<h2 id="architecture">Architecture</h2>
<p>Application routes, auth, publish, and docs wiring live in <code>app/*.nex</code>. The <strong>nex</strong> CLI (from the sibling <code>nex-lang</code> project) provides the interpreter, PostgreSQL driver, HTTP listener, templates, and host builtins (<code>http_*</code>, <code>db_*</code>, <code>fs_*</code>, <code>json_*</code>, crypto, OIDC).</p>

<h2 id="syntax">Core syntax</h2>
<pre><code>import "helpers.nex";

let greet = fn(name: string) -> string {
  return "hello " + name;
};

http_get("/healthz", fn(req) {
  return json({"status": "ok"});
});</code></pre>
<p>Also supported: <code>if/else</code>, <code>while</code>, <code>return</code>, <code>break</code>/<code>continue</code>, arrays <code>[]</code>, maps <code>{}</code>, index/assign, <code>&amp;&amp;</code>/<code>||</code>, comments.</p>

<h2 id="types">Gradual type annotations</h2>
<p>Optional annotations on <code>let</code> bindings and function parameters/returns are checked at runtime for common types (<code>int</code>, <code>string</code>, <code>bool</code>, <code>array</code>, <code>hash</code>/<code>result</code>, <code>fn</code>). Unknown type names are accepted (documentation-only).</p>
<pre><code>let n: int = 7;
let add = fn(a: int, b: int) -> int { a + b };</code></pre>

<h2 id="structs">Structs &amp; field access</h2>
<pre><code>struct Point { x, y };
let p = Point(3, 4);
puts(p.x + p.y); // 7</code></pre>
<p>Constructors return hashes tagged with <code>__struct</code>. Use <code>.</code> for field access (same as string-key lookup).</p>

<h2 id="match">Pattern matching</h2>
<pre><code>let label = match (code) {
  200 -> "ok",
  404 -> "missing",
  _ -> "other"
};</code></pre>
<p>Arms use <code>-&gt;</code>. Literals compare by value; <code>_</code> is a wildcard; other identifiers bind the matched value.</p>

<h2 id="result">Result &amp; <code>try</code></h2>
<p>Results are hashes: <code>ok(value)</code> / <code>err(message)</code> with helpers <code>is_ok</code>, <code>is_err</code>, <code>unwrap</code>.</p>
<pre><code>let parse = fn(n) {
  if (n &gt; 0) { return ok(n); }
  return err("bad");
};

let run = fn(n) {
  let v = try parse(n); // early-returns the Err Result from the function
  return v * 2;
};</code></pre>

<h2 id="pipe">Pipes &amp; collection helpers</h2>
<pre><code>5 |&gt; fn(x) { x * 2 };
[1, 2, 3] |&gt; map(fn(x) { x + 1 });
[1, 2, 3, 4] |&gt; filter(fn(x) { x % 2 == 0 });</code></pre>
<p><code>|&gt;</code> inserts the left value as the first argument to a function or call.</p>

<h2 id="modules">Modules</h2>
<p><code>import "helpers.nex"</code> loads a sibling file into the current environment (used heavily by the registry’s <code>app/*.nex</code> tree).</p>

<h2 id="stdlib">Host stdlib (registry-facing)</h2>
<ul>
  <li><strong>HTTP</strong> — <code>http_get</code>, <code>http_post</code>, request helpers, <code>json</code>, <code>html</code>, <code>redirect</code></li>
  <li><strong>DB</strong> — <code>db_*</code> package/user/OIDC helpers backed by Postgres</li>
  <li><strong>FS / crypto</strong> — file storage, hashing, API key prefixes</li>
  <li><strong>Core</strong> — <code>len</code>, <code>str</code>, <code>int</code>, <code>type</code>, <code>push</code>, <code>keys</code>, <code>split</code>/<code>join</code>, Result helpers, <code>map</code>/<code>filter</code></li>
</ul>

<h2 id="run">Run the registry</h2>
<pre><code>$env:DATABASE_URL = "postgres://postgres:postgres@localhost:5432/nex_registry?sslmode=disable"
$env:NEX_WEB_DIR = ".\web"
..\nex-lang\bin\nex.exe run .\app\main.nex</code></pre>
<p>Entrypoint loads <code>app/main.nex</code>, which registers all HTTP routes on <code>LISTEN_ADDR</code> (default <code>:8080</code>). See also <code>nex-lang/examples/features.nex</code> for a language tour.</p>
`),
	},
}
