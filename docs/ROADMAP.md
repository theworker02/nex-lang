# Nexus roadmap

## Implemented (current)

- **TypeScript toolchain** (`vscode-nexus/`): tree-walk eval, bytecode VM, REPL, tests, modules, stdlib, package client, WASM/LLVM text codegen, VS Code extension
- **Self-host bootstrap**: `selfhost/*.nex` lexer/parser/evaluator run via `node out/cli.js selfhost`
- **Web host (TS)**: serve `nex-registry` browse/docs/search demos with in-memory DB (`npm run registry`)
- **Design language**: declarative theme/layout/components in `.nex` → HTML/CSS; registry `/design` + `npm run site`
- **Documentation**: `vscode-nexus/docs/` language + toolchain + design reference; Keep-a-Changelog files
- **Go CLI** (legacy/compat): tree-walk + VM + Postgres host + `try` keyword

## Next phase (opinionated order)

1. **Unify engines** — lower `import` + selected host calls into the VM (or hybrid: tree-walk modules, VM for hot functions); make `--vm` competitive for non-HTTP scripts once parity is closer.
2. **Static diagnostics** — shared checker for CLI and editor (undefined names, arity, gradual types) so the editor is not a second language.
3. **Export/import bindings** — stop merging modules into one global env; introduce `export` / selective import so packages compose without name clashes.
4. **Design language depth** — more components (forms, tables), dark-token themes, optional compile-to-static export; migrate more registry marketing surfaces from HTML templates into `.nex` design modules.
5. **Self-host expansion** — structs, pipes, Results, then in-nex `import`; emit a stable AST/bytecode format; shrink the TS surface toward a loader + host FFI.
6. **TS `try` parity** — port Go Result early-return if we want one language surface across hosts.
7. **Registry host depth** — replace TS `db_*` stubs with a real persistence story (or document clearly that production registry remains Go+Postgres).

## Non-goals (for now)

- Claiming effects/regions/macros/async are production-ready
- Documenting unimplemented fantasy features as done
- Requiring Go to develop Nexus day-to-day
