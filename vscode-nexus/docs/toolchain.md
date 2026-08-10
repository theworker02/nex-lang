# Nexus TypeScript toolchain

The language core lives under `vscode-nexus/` (TypeScript). Go sources under `cmd/` / `pkg/` are **not** required to run, test, extend Nexus, or serve the registry website from this extension.

Full documentation index: [README.md](README.md).

## Capabilities

| Feature | How |
| --- | --- |
| Tree-walk runtime | Default `nexus.run` / `node out/cli.js run` |
| Bytecode VM | Setting `nexus.executionEngine` = `vm`, command **Run File (Bytecode VM)**, or `node out/cli.js run file.nex --vm` |
| Modules | `import "strings";` — local paths, `.modules/`, `stdlib/` |
| Tests | **Nexus: Run Tests** or `npm run test:nex` |
| REPL | **Nexus: Open REPL** or `npm run repl` |
| Self-host | **Nexus: Run File (Self-hosted)** or `node out/cli.js selfhost` |
| Stdlib | `stdlib/strings.nex`, `stdlib/result.nex` + TS builtins |
| HTTP web host | Routes + templates + static files + memory DB |
| Registry website | `npm run registry` → sibling `nex-registry/app/*.nex` |
| WASM / LLVM | Editor commands write `.wat` / `.ll` for a core subset |
| Packages | Editor **Publish** / **Install** via `RegistryClient` |

## CLI (Node, no Go)

```bash
cd vscode-nexus
npm run compile
node out/cli.js run examples/modules_demo.nex
node out/cli.js run examples/vm_demo.nex --vm
node out/cli.js selfhost examples/selfhost_demo.nex
npm run test:nex
npm run repl
npm run smoke
npm run docs
```

```text
nex-ts — Nexus language toolchain (TypeScript)

Commands:
  run <file.nex> [--vm] [--no-serve]   Execute (serves HTTP if routes register)
  selfhost <file.nex>                  Self-hosted .nex lexer/parser/evaluator
  repl [--vm]                          Interactive REPL
  test [paths...]                      Run *_test.nex / tests/**/*.nex
  help                                 Show help (includes docs path)
  version                              Show version (nex-ts 0.3.0)
```

Docs on disk: `vscode-nexus/docs/` (start at `docs/README.md`).

## Engine tiers

| Tier | Purpose |
| --- | --- |
| `eval` | Full tree-walk + modules + stdlib install + optional web host |
| `vm` | Bytecode compile + stack VM; core language |
| `wasm` / `native` | Emit WebAssembly text (`.wat`); native path adds `NATIVE.md` instructions |
| `llvm` | Emit LLVM IR text (`.ll`) |

Codegen covers a **subset** of constructs (lets, arithmetic, control flow, simple functions, match/ctors). It is not a complete ahead-of-time product yet.

## Module resolution order

1. Absolute path  
2. Relative to importing file (`__dir__`)  
3. Workspace / app root  
4. `.modules/<name>`  
5. `stdlib/` (extension `stdlib/` or repo-root `stdlib/`)

Candidates: `path.nex`, `path/mod.nex`, `path/main.nex`.

## VM limits

The bytecode VM covers core language (lets, functions, arrays, hashes, control flow, builtins). **`import` is tree-walk only** — use `eval` when modules are needed (required for the registry app).

## Run the Nexus Registry website

```powershell
cd vscode-nexus
npm run compile
npm run registry
# → http://localhost:8080
```

Details: [website.md](website.md).

Without Postgres, the TypeScript host uses an **in-memory demo DB**. Full Postgres-backed auth/publish remains available via the Go `nex` host when `DATABASE_URL` is set.

## Self-hosting

See [selfhosting.md](selfhosting.md) and [`../selfhost/README.md`](../selfhost/README.md).

```bash
npm run compile
node out/cli.js selfhost examples/selfhost_demo.nex
```

## Editor settings

| Setting | Default | Meaning |
| --- | --- | --- |
| `nexus.registryUrl` | `http://localhost:8080` | Package registry base URL |
| `nexus.diagnostics.enable` | `true` | Parse / ownership / effect diagnostics |
| `nexus.compileOutputDir` | `""` → `<workspace>/nex-out` | WASM/LLVM output |
| `nexus.compileTarget` | `eval` | Default compile tier preference |
| `nexus.executionEngine` | `eval` | `eval` or `vm` for Run File |

## npm scripts

| Script | Action |
| --- | --- |
| `compile` | `tsc` |
| `watch` | `tsc -watch` |
| `lint` | typecheck only |
| `smoke` | language + ultimate + registry + toolchain smokes |
| `test:nex` | `node out/cli.js test` |
| `repl` | open REPL |
| `run:nex` | `node out/cli.js run` |
| `selfhost` | `node out/cli.js selfhost` |
| `registry` | start nex-registry on TS host |
| `docs` | print absolute path to this docs tree |
| `package` | `vsce package` |
