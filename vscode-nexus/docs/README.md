<p align="center">
  <img src="../media/logo.png" alt="Nexus" width="120" height="120">
</p>

# Nexus documentation

Primary documentation for the **TypeScript Nexus toolchain** (`vscode-nexus/`). This is the supported way to run, test, self-host, and serve Nexus apps (including the package registry website and design-language sites).

A legacy Go CLI still exists under the monorepo root (`cmd/nex`, `pkg/*`). Prefer these pages unless you specifically need Go-only features (notably the `try` keyword and historically Postgres-backed host paths).

**Brand:** docs and UIs should show the [ribbon N mark](../media/logo.png) (`media/logo.svg` / `logo.png`) **without** baking the word “NEXUS” into the image. Pair the mark with separate text (“Nexus”, “Nexus Registry”, etc.).

---

## Start here

| If you want to… | Read |
| --- | --- |
| Install and run your first program | [Getting started](getting-started.md) |
| Understand the language surface | [Language overview](language/overview.md) |
| Author UI in `.nex` | [Design language](design/README.md) · [Design guide](design/guide.md) |
| Learn CLI / engines / editor settings | [Toolchain](toolchain.md) |
| Run the registry website | [Website apps](website.md) |
| Publish or install packages | [Packages & registry](packages.md) |
| See self-host status | [Self-hosting](selfhosting.md) · [`../selfhost/README.md`](../selfhost/README.md) |

```bash
cd vscode-nexus
npm run compile
npm run docs          # print absolute path to this docs tree
node out/cli.js help
```

---

## Contents

### Orientation

| Document | Topic |
| --- | --- |
| [Getting started](getting-started.md) | Node setup, compile, run, REPL, tests, editor F5 |
| [Examples](examples.md) | Demo programs index |
| [Toolchain](toolchain.md) | Engine tiers, CLI, WASM/LLVM, module resolution, settings |

### Language reference

| Document | Topic |
| --- | --- |
| [Language overview](language/overview.md) | What Nexus is today |
| [Syntax & expressions](language/syntax.md) | Lexical rules, operators, sugar |
| [Types](language/types.md) | Gradual annotations, values |
| [Control flow](language/control-flow.md) | `if`, `while`, `for`, `break` / `continue` |
| [Functions](language/functions.md) | Closures, calls, pipes |
| [Modules](language/modules.md) | `import` resolution order |
| [Match & ADTs](language/match.md) | `match`, `struct`, `enum` |
| [Concurrency & effects](language/concurrency-effects.md) | What exists vs experimental |

### Runtime & libraries

| Document | Topic |
| --- | --- |
| [Builtins](builtins.md) | Core + host-provided builtins |
| [Stdlib](stdlib.md) | Shipped `.nex` modules (`strings`, `result`, `design`, …) |

### Tooling platforms

| Document | Topic |
| --- | --- |
| [Self-hosting](selfhosting.md) | Bootstrap architecture, supported subset, host substrate |
| [Packages & registry](packages.md) | `nexus.toml`, publish/install client |
| [Website apps](website.md) | Serving `nex-registry` on the TS host; env vars; memory DB |

### Design language

| Document | Topic |
| --- | --- |
| [Design README](design/README.md) | Philosophy, API, host builtins, registry routes |
| [Design guide](design/guide.md) | Walkthrough for authoring UI in `.nex` |

---

## Toolchain at a glance

```text
                    ┌─────────────┐
   .nex source ───► │  nex-ts CLI │──► eval (default) ──► modules + host + HTTP
                    │  extension  │──► vm (--vm)       ──► core bytecode only
                    └──────┬──────┘──► selfhost        ──► selfhost/*.nex pipeline
                           │
                           ├── design_* builtins ──► HTML/CSS (+ nxd.js)
                           └── RegistryClient    ──► publish / install
```

| Capability | How |
| --- | --- |
| Tree-walk runtime | `node out/cli.js run file.nex` |
| Bytecode VM | `run file.nex --vm` |
| Self-host | `selfhost file.nex` |
| Tests | `npm run test:nex` |
| REPL | `npm run repl` |
| Registry site | `npm run registry` → `http://localhost:8080` |
| Design demo | `npm run site` → `http://localhost:8090` |

---

## Honesty checklist

Keep these constraints visible when extending docs or demos:

- Prefer **documented, implemented** behavior over aspirational roadmap language.
- VM path: **no `import`**, no web-host builtins.
- Self-hosted evaluator: deliberate **subset** (see selfhosting docs).
- TS web host DB: **in-memory** demo by default; setting `DATABASE_URL` currently warns and does not switch `db_*` to Postgres on the TS host.
- `try` early-return: **Go-only**; TS uses `is_ok` / `unwrap`.
- Effects / ownership / codegen: **experimental**.

---

## Related changelogs & specs

| Link | Role |
| --- | --- |
| [../CHANGELOG.md](../CHANGELOG.md) | Toolchain changelog |
| [../../CHANGELOG.md](../../CHANGELOG.md) | Monorepo changelog |
| [../../docs/spec.md](../../docs/spec.md) | Implemented language surface |
| [../../docs/ROADMAP.md](../../docs/ROADMAP.md) | Near-term priorities |
| [../../README.md](../../README.md) | Monorepo overview + architecture diagram |
| [../README.md](../README.md) | Extension + CLI deep dive |

```bash
npm run docs
```
