<p align="center">
  <img src="media/logo.png" alt="Nex LSP" width="140" height="140">
</p>

<h1 align="center">Nex LSP</h1>

<p align="center">
  Language support extension for Nexus (<code>.nex</code>) — VS Code / VSCodium / Open VSX.<br>
  Ships with the TypeScript Nexus toolchain in this package.
</p>

<p align="center">
  <strong><a href="https://open-vsx.org/extension/theworker02/nex-lsp">Install on Open VSX</a></strong> ·
  <a href="docs/README.md">Documentation hub</a> ·
  <a href="docs/getting-started.md">Getting started</a> ·
  <a href="CHANGELOG.md">Changelog</a> ·
  <a href="../README.md">Monorepo README</a>
</p>

---

## Brand

Extension and docs use the **ribbon N mark only** (no “NEXUS” wordmark inside the image). UI chrome places the mark beside separate text labels (e.g. “Nexus Registry”).

| File | Use |
| --- | --- |
| [`media/logo.svg`](media/logo.svg) / [`media/logo.png`](media/logo.png) | README + brand mark (full-res ribbon N) |
| [`media/logo-256.png`](media/logo-256.png) | **Nex LSP** marketplace / Open VSX icon (`package.json` → `icon`) |
| [`media/nex-file-dark.svg`](media/nex-file-dark.svg) / [`nex-file-light.svg`](media/nex-file-light.svg) | Editor file icon for `.nex` |

---

## What this package is

One npm / VSIX package that includes:

1. **Language core** — lexer, parser, tree-walk evaluator, diagnostics  
2. **Bytecode VM** — compile + stack machine (`run --vm`)  
3. **Self-hosted pipeline** — `selfhost/*.nex` lex → parse → eval  
4. **CLI** — `out/cli.js` (`nex-ts`): `run` / `repl` / `test` / `selfhost` / `help` / `version`  
5. **HTTP web host** — routes, templates, static files, design→HTML, in-memory demo DB  
6. **Package client** — `nexus.toml`, archive, publish, install  
7. **Design language** — `stdlib/design.nex` + host render builtins  
8. **Editor integration** — highlighting, snippets, commands, settings  
9. **Experimental codegen** — WASM text / LLVM IR text for a core subset  

Legacy Go CLI lives at the monorepo root (`cmd/nex`). Prefer this TypeScript toolchain unless you need Go-only behavior (Postgres-backed host historically, `try` keyword).

---

## Quick start

```bash
npm install
npm run compile

node out/cli.js run examples/modules_demo.nex
node out/cli.js run examples/vm_demo.nex --vm
node out/cli.js selfhost examples/selfhost_demo.nex

npm run test:nex
npm run repl
npm run smoke
```

```bash
npm run docs          # absolute path to docs/
node out/cli.js help
node out/cli.js version   # e.g. nex-ts 0.3.0
```

### Language website (self-contained)

```bash
npm run site          # http://localhost:8090 — no registry required
npm run build:site    # static export for GitHub Pages
```

### Optional local registry

Requires a sibling checkout of `nex-registry` (not published with this repo):

```bash
npm run registry
# → http://localhost:8080
```

---

## Documentation map

| Document | Topic |
| --- | --- |
| [docs/README.md](docs/README.md) | Full index |
| [docs/getting-started.md](docs/getting-started.md) | Install → run → editor |
| [docs/language/overview.md](docs/language/overview.md) | Language today |
| [docs/language/syntax.md](docs/language/syntax.md) | Lexical rules & sugar |
| [docs/language/types.md](docs/language/types.md) | Gradual types |
| [docs/language/control-flow.md](docs/language/control-flow.md) | `if` / loops |
| [docs/language/functions.md](docs/language/functions.md) | Closures, pipes |
| [docs/language/modules.md](docs/language/modules.md) | `import` resolution |
| [docs/language/match.md](docs/language/match.md) | `match`, `struct`, `enum` |
| [docs/language/concurrency-effects.md](docs/language/concurrency-effects.md) | Experimental surface |
| [docs/builtins.md](docs/builtins.md) | Core + host builtins |
| [docs/stdlib.md](docs/stdlib.md) | Shipped `.nex` modules |
| [docs/toolchain.md](docs/toolchain.md) | Engines, CLI, editor settings |
| [docs/selfhosting.md](docs/selfhosting.md) | Bootstrap & subset |
| [selfhost/README.md](selfhost/README.md) | Selfhost tree notes |
| [docs/packages.md](docs/packages.md) | `nexus.toml`, publish/install |
| [docs/website.md](docs/website.md) | Optional local registry hosting |
| [docs/site.md](docs/site.md) | Language homepage + GitHub Pages |
| [docs/design/README.md](docs/design/README.md) | Design language |
| [docs/design/guide.md](docs/design/guide.md) | Authoring walkthrough |
| [docs/examples.md](docs/examples.md) | Demo index |

---

## Layout

| Path | Role |
| --- | --- |
| `src/language/` | Lexer, parser, evaluator, builtins, diagnostics |
| `src/vm/` | Bytecode compiler + VM |
| `src/compiler/` | Multi-tier engine + WASM/LLVM codegen |
| `src/host/` | HTTP, templates, memory DB, design render |
| `src/registry/` | Package publish/install client |
| `src/cli.ts` | CLI entry |
| `src/extension.ts` | VS Code extension activation & commands |
| `selfhost/` | Nexus-written lexer/parser/evaluator |
| `stdlib/` | Importable `.nex` modules (`strings`, `result`, `design`, …) |
| `media/` | Logo mark + marketplace / file icons |
| `docs/` | Language & toolchain documentation |
| `examples/` | Demos (`examples/site` for design) |
| `tests/` | `nex` test files |
| `scripts/` | `registry`, `site`, smoke, docs path helpers |
| `syntaxes/` · `snippets/` | TextMate grammar + snippets |

---

## CLI reference

```text
nex-ts — Nexus language toolchain (TypeScript)

Commands:
  run <file.nex> [--vm] [--no-serve]   Execute (serves HTTP if routes register)
  selfhost <file.nex>                  Self-hosted .nex lexer/parser/evaluator
  repl [--vm]                          Interactive REPL
  test [paths...]                      Run *_test.nex / tests/**/*.nex
  help                                 Show help (includes docs path)
  version                              Show version
```

| Flag / behavior | Meaning |
| --- | --- |
| Default `run` | Tree-walk (`eval`) — modules + host builtins |
| `--vm` | Bytecode VM — core language only |
| `--no-serve` | Do not start HTTP even if the program registers routes |
| `NEX_FORCE_SERVE=1` | Force serve behavior when applicable |

### HTTP / registry environment variables

Used when a program (or `npm run registry` / `npm run site`) starts the web host:

| Variable | Default | Purpose |
| --- | --- | --- |
| `NEX_APP_DIR` | Directory of entry `.nex` | App root for imports / routes |
| `NEX_WEB_DIR` | (optional) | Path to `web/` (templates + `static/`) |
| `LISTEN_ADDR` | `:8080` | Bind address (`npm run site` uses 8090 via script) |
| `BASE_URL` | `http://localhost:8080` | Public base URL (cookies / links) |
| `CDN_BASE_URL` | same as `BASE_URL` | Asset base |
| `STORAGE_DIR` | `<app-parent>/storage` | Package artifacts / seed data |
| `MAX_UPLOAD_BYTES` | 64 MiB | Upload limit |
| `DATABASE_URL` | unset | When set, TS host tries Postgres (`pg`); on failure falls back to **in-memory** demo DB |
| `MIGRATIONS_DIR` | `<app-parent>/migrations` | Reserved for migration paths |
| `COOKIE_SECURE` | derived from `BASE_URL` | Force `1`/`true` or `0`/`false` |
| `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET` | unset | Optional OAuth wiring when present |

**Honest DB note:** without `DATABASE_URL`, the TypeScript host uses an **in-memory** store (optionally seeded from `STORAGE_DIR`). With `DATABASE_URL`, it uses Postgres when reachable and falls back to memory if init fails.

---

## Language website

```bash
npm run site          # http://localhost:8090 (no registry required)
npm run build:site    # static HTML → ../site/ for GitHub Pages
```

Docs: [docs/site.md](docs/site.md). Public Pages: https://theworker02.github.io/nex-lang/

### Registry website (optional local sibling)

Requires a sibling checkout of `nex-registry` (not published with this repo):

```bash
npm run registry
# → http://localhost:8080
```

## Engine tiers

| Tier | How to invoke | Strengths | Limits |
| --- | --- | --- | --- |
| `eval` | default `run` / editor Run File | Full language, modules, HTTP host | — |
| `vm` | `--vm` / Run File (Bytecode VM) | Faster core loops | No `import`, no web builtins |
| `selfhost` | `selfhost` / Run File (Self-hosted) | Pipeline in `.nex` | Subset evaluator |
| `wasm` / `llvm` / `native` | Compile commands | Emit IR / notes | Core subset only |

Module resolution (eval): absolute → relative to importer → workspace/app root → `.modules/<name>` → `stdlib/`. Candidates: `path.nex`, `path/mod.nex`, `path/main.nex`.

---

## Editor (VS Code / VSCodium / Open VSX)

**Install from Open VSX:** [Nex LSP](https://open-vsx.org/extension/theworker02/nex-lsp) (`theworker02.nex-lsp`)

From source / VSIX:

1. Open this folder (or the monorepo) and press **F5**, or `npm run package` for a VSIX.  
2. Open a `.nex` file — syntax highlighting + snippets activate.  
3. Command Palette → **Nexus:** …

| Command | ID | Typical use |
| --- | --- | --- |
| Run File | `nexus.run` | Tree-walk execute |
| Run File (Bytecode VM) | `nexus.runVm` | VM execute |
| Run File (Self-hosted) | `nexus.selfhost` | Selfhost pipeline |
| Run Tests | `nexus.test` | Test runner |
| Open REPL | `nexus.repl` | Interactive |
| Compile to WASM / LLVM / Native | `nexus.compile*` | Emit artifacts |
| Publish / Install Package | `nexus.publish` / `nexus.install` | Registry client |

### Settings

| Setting | Default | Meaning |
| --- | --- | --- |
| `nexus.registryUrl` | `http://localhost:8080` | Package registry base URL |
| `nexus.diagnostics.enable` | `true` | Parse / ownership / effect diagnostics |
| `nexus.compileOutputDir` | `<workspace>/nex-out` | Codegen output |
| `nexus.compileTarget` | `eval` | Default compile tier |
| `nexus.executionEngine` | `eval` | `eval` or `vm` for Run File |

---

## Design system & self-designed UI

- Stdlib builders: theme, stack/row/grid, brand/nav, forms, tables, footer/sidebar, …  
- Host: `design_document` / `design_response` / `design_render` / `design_css`  
- Client kit: `nxd.js` (search shortcut, copy buttons, drawers) inlined by `design_document`  
- Registry browse/auth/docs/legal pages are largely design-authored when served from the TS host  

Brand nodes take `mark` (URL to **logo image**) plus `name` / `tag` **text** — keep wordmarks out of the PNG/SVG.

Details: [docs/design/README.md](docs/design/README.md).

---

## Self-hosting

```bash
node out/cli.js selfhost examples/selfhost_demo.nex
```

Supported subset and host-provided `fs_read` / `puts` / `__argv__`: [docs/selfhosting.md](docs/selfhosting.md), [selfhost/README.md](selfhost/README.md).

---

## npm scripts

| Script | Action |
| --- | --- |
| `compile` / `watch` / `lint` | TypeScript build |
| `test:nex` | Nexus test runner |
| `repl` / `run:nex` / `selfhost` | CLI wrappers |
| `registry` / `site` / `build:site` | Optional registry / live language site / static Pages export |
| `smoke` | Language + registry + design smoke scripts |
| `docs` | Print docs path |
| `package` | `vsce package` VSIX — **Nex LSP** (`theworker02.nex-lsp`) |

---

## Honest substrate & known limits

- Host builtins (FS, HTTP, crypto, DB APIs, …) are implemented in TypeScript.  
- Bytecode VM does not support `import` or web-host builtins.  
- Self-hosted evaluator is a deliberate subset (no modules/structs/pipes/Results/effects).  
- `try` Result early-return is Go-only; use `is_ok` / `unwrap` on TS.  
- Many advanced `db_*` / auth paths are demo-grade on the TS memory host.  
- Ownership, effects, and codegen are experimental — not production guarantees.  
- See [docs/builtins.md](docs/builtins.md) and the changelog “Known limitations”.

---

## Legacy Go CLI

Monorepo root:

```powershell
go build -o ..\bin\nex.exe .\cmd\nex   # from repo root: go build -o bin/nex.exe ./cmd/nex
..\bin\nex.exe run .\examples\features.nex
```

Use Go when you specifically need its Postgres host path or `try`. Day-to-day language work should stay on this TypeScript package.
