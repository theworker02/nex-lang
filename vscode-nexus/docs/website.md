# Website apps (registry on the TS host)

The Nexus Registry website is largely written in `.nex` (`nex-registry/app/*.nex`) and can be served by the **TypeScript** host — no Go binary required for browse/docs/search demos.

## Quick start

Expect a sibling checkout:

```text
personal projects/
  nex-lang/
    vscode-nexus/
  nex-registry/
    app/main.nex
    web/
    storage/
```

```powershell
cd vscode-nexus
npm run compile
npm run registry
# → http://localhost:8080
```

Override location:

```powershell
$env:NEX_REGISTRY_DIR = "C:\path\to\nex-registry"
npm run registry
```

## Design language pages

Major registry UI is authored in the Nexus Design Language (`design_response`), not Go templates:

| URL | Source |
| --- | --- |
| `/`, `/search`, `/packages`, `/packages/{name}` | `app/design/registry_pages.nex` |
| `/docs`, `/docs/{page}` | docs layout + sidebar |
| `/login`, `/register`, `/settings` | auth design pages |
| `/legal/*` | legal design pages |
| `/design`, `/design/guide` | design language showcase |

Routes live under `app/design/` and are wired from `web.nex` / `auth.nex` / `legal.nex` / `design/routes.nex`.

Standalone demo (no full registry app):

```powershell
npm run site
# → http://localhost:8090
```

See [Design language](design/README.md).

## Explicit run

```powershell
$env:NEX_WEB_DIR = (Resolve-Path ..\..\nex-registry\web).Path
$env:STORAGE_DIR = (Resolve-Path ..\..\nex-registry\storage).Path
$env:LISTEN_ADDR = ":8080"
$env:BASE_URL = "http://localhost:8080"
$env:NEX_APP_DIR = (Resolve-Path ..\..\nex-registry\app).Path
node out/cli.js run ..\..\nex-registry\app\main.nex
```

(Adjust relative paths if your layout differs.)

## Environment variables

| Variable | Role |
| --- | --- |
| `NEX_WEB_DIR` | Templates + static assets (fonts/images; CSS often from design theme) |
| `STORAGE_DIR` | Package artifacts / seed data |
| `LISTEN_ADDR` | Default `:8080` |
| `BASE_URL` | Public base URL (`https://` enables Secure cookies) |
| `COOKIE_SECURE` | Force `1`/`0` for session cookie Secure flag |
| `DATABASE_URL` | When set, TS host uses real Postgres for `db_*` |
| `MIGRATIONS_DIR` | SQL migrations applied on Postgres startup (default: sibling `migrations/`) |
| `NEX_APP_DIR` | App root for imports |
| `NEX_FORCE_SERVE` | Force listen even if no routes registered |
| `--no-serve` CLI flag | Run without starting HTTP |

## How routing works

Nexus code registers handlers with host builtins:

```nex
http_get("/", fn(req) {
  return design_response(home_design(req, featured, recent, stats));
});
```

When `routeCount > 0`, the CLI keeps the process alive and listens. Request objects expose `method`, `path`, `query`, `headers`, `params`, `cookies`, `form`, `body`, `user`, etc.

## DB modes (TS host)

| Mode | When | Capability |
| --- | --- | --- |
| Memory | `DATABASE_URL` unset | In-memory demo DB seeded from `storage/`; browse, docs, search, register/login sessions in process |
| Postgres | `DATABASE_URL` set | Real `pg` driver; migrations; users, sessions (hashed), packages, auth tokens, API keys, orgs |

```powershell
$env:DATABASE_URL = "postgres://nexus:nexus@localhost:5432/nexus"
npm run registry
```

`GET /healthz` includes `db_mode`. Publish/yank and some admin APIs may still be incomplete on TS vs Go — but auth and package reads are real against Postgres when configured.

## Writing your own HTTP app

1. Create `app/main.nex` that calls `http_get` / `http_post` / …
2. Prefer `design_response` for UI; point `NEX_WEB_DIR` at static assets.
3. `node out/cli.js run app/main.nex`
4. Stay on the **eval** engine (needs modules + host builtins).
