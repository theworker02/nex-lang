# Buyer evaluation â€” Run programs

## Goal

In 15â€“45 minutes, verify the Product builds or runs as documented and that proprietary notices are present.

## Steps

1. Confirm root `LICENSE` is proprietary and `ACQUISITION.md` exists.
2. Skim `README.md` install/run claims.
3. Execute:

```
```powershell
cd vscode-nexus
npm install
npm run compile

# Run programs
node out/cli.js run .\examples\modules_demo.nex
node out/cli.js run .\examples\vm_demo.nex --vm
node out/cli.js selfhost .\examples\selfhost_demo.nex

# REPL + tests
npm run repl
npm run test:nex

# Language homepage (self-contained Ã¢â‚¬â€ no registry)
npm run site
# Ã¢â€ â€™ http://localhost:8090

# Static export for GitHub Pages
npm run build:site
# Ã¢â€ â€™ ../site/
```
```text
nex-lang/
Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ assets/                 # Brand mark (logo.svg / logo.png) Ã¢â‚¬â€ mark only
Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ vscode-nexus/           # Ã¢Ëœâ€¦ Primary TS toolchain + VS Code extension + docs
Ã¢â€â€š   Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ src/language/       # Lexer, parser, evaluator, builtins, diagnostics
Ã¢â€â€š   Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ src/vm/             # Bytecode compiler + stack VM
Ã¢â€â€š   Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ src/compiler/       # Multi-tier engine + WASM/LLVM text codegen
Ã¢â€â€š   Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ src/host/           # HTTP host, templates, designÃ¢â€ â€™HTML, memory DB
Ã¢â€â€š   Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ src/registry/       # Package publish/install client
Ã¢â€â€š   Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ src/cli.ts          # run / repl / test / selfhost
Ã¢â€â€š   Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ selfhost/           # .nex lexer / parser / evaluator
Ã¢â€â€š   Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ stdlib/             # Importable .nex modules (incl. design.nex)
Ã¢â€â€š   Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ media/              # Extension icons + logo
Ã¢â€â€š   Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ docs/               # Language & toolchain documentation
Ã¢â€â€š   Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ examples/           # Demos (+ examples/site design demo)
Ã¢â€â€š   Ã¢â€â€Ã¢â€â‚¬Ã¢â€â‚¬ tests/              # *_test.nex
Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ packages/sdk/           # TypeScript registry control client (`@theworker02/nex-sdk`)
Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ cmd/nex, pkg/*          # Legacy Go CLI / host
```

4. Run tests if present (`npm test`, `pytest`, `cargo test`, `go test ./...`, etc.).
5. Record README vs observed behavior gaps in workpapers.

## Pass criteria

- [ ] Clone succeeds
- [ ] Documented happy path works **or** failure is explained
- [ ] Minimal path needs no surprise secrets
- [ ] License notices intact

*Updated: 2026-09-22*
