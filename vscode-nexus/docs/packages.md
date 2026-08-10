# Packages and registry

## Manifest: `nexus.toml`

```toml
name = "httpkit"
version = "1.0.0"
author = "you"
description = "HTTP helpers for Nexus"
dependencies = {}
```

Validation (TypeScript client):

- `name`: `[a-zA-Z][a-zA-Z0-9_-]{0,63}`
- `version`: semver-like `X.Y.Z` with optional prerelease suffix

Constants: `MANIFEST_FILENAME = "nexus.toml"`, installs land in `.modules/`.

## Client API / editor commands

`src/registry/client.ts` implements:

- Read/write manifest
- Create `.tar.gz` archives (skips `.git`, `.modules`, the archive itself)
- Publish to the registry HTTP API
- Install by name/version into `.modules/`
- Login / token helpers as supported by the registry

In the editor:

- **Nexus: Publish Package** — uses workspace root + `nexus.registryUrl`
- **Nexus: Install Package** — prompts for a package spec

Default registry URL: `http://localhost:8080` (setting `nexus.registryUrl`).

## Recommended layout

```text
nexus.toml
src/ or app/        # application .nex sources
stdlib/             # optional local stdlib overlay
.modules/           # installed dependencies (generated)
tests/              # *_test.nex or tests/**/*.nex
examples/           # demos
```

## Consuming installs

After install, import by package name:

```nex
import "httpkit";
```

Resolution searches `.modules/` — see [language/modules.md](language/modules.md).

## Serving a registry

Local demo (TS host, in-memory DB):

```bash
cd vscode-nexus
npm run registry
```

See [website.md](website.md). Publishing and authenticated flows that need Postgres still prefer the Go host + real database.
