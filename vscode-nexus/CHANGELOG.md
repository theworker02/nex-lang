# Changelog

All notable changes to the Nexus TypeScript toolchain (`vscode-nexus`) are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to follow [Semantic Versioning](https://semver.org/) for the language CLI (`nex-ts`) and extension package where practical.

## [Unreleased]

## [0.2.0] - 2026-08-11

Extension package version **0.2.0**. Language / CLI version reported by `node out/cli.js version` is **nex-ts 0.4.0**.

### Added

- `vscode-nexus/src/language/static_check.ts` with `checkProgram` for undefined-name, arity-mismatch, and unused-local diagnostics.
- Diagnostics pipeline wires static checks alongside ownership and effect analysis.
- `nex-ts check <file.nex>` CLI subcommand.
- `scripts/smoke-static-check.js` smoke coverage.

## [0.1.1] - 2026-08-10

### Changed

- Patch release metadata alignment for Open VSX publishing.

## [0.3.0] - 2026-08-09

Language / CLI version reported by `node out/cli.js version` (`nex-ts 0.3.0`). Extension package version remains `0.1.0` until a marketplace cut.

### Added

- **Tree-walk evaluator** with modules (`import`), gradual types, structs, enums, match (`->` / `=>`), pipes (`|>`), Results (`ok`/`err`/`unwrap`), and English syntax sugar.
- **Bytecode compiler + stack VM** (`run --vm`, REPL `--vm`) for core language programs.
- **Self-hosted pipeline** in `selfhost/*.nex` (lexer, parser, evaluator, `main.nex`) bootstrapped by the TS host (`selfhost` command / editor action).
- **Stdlib `.nex` modules**: `strings`, `result`, plus Node-installed helpers (FS, crypto, net, task, memory, optional FFI).
- **Web host** sufficient to serve `nex-registry` browse/docs/search demos with an in-memory DB (`npm run registry`).
- **Package client**: `nexus.toml` read/write, archive, publish, install (editor commands + `RegistryClient`).
- **WASM / LLVM text codegen** for a core subset (editor compile commands).
- **Test runner** (`test` / `npm run test:nex`) and **REPL**.
- VS Code / Open VSX extension: highlighting, snippets, diagnostics hooks, run/test/repl/selfhost/compile/package commands.
- Experimental grammar for effects, regions, reflection, macros, async/spawn/chan (partial runtime).

### Changed

- Primary development story moved to the TypeScript toolchain; Go CLI remains available as legacy/compat (Postgres host, `try` keyword).

### Known limitations

- Bytecode VM does not support `import` or web-host builtins.
- Self-hosted evaluator is a deliberate subset (no modules/structs/pipes/Results/effects).
- `try` Result early-return is Go-only; use `is_ok` / `unwrap` on TS.
- Many `db_*` publish/auth APIs are stubs on the TS demo host.
- Ownership, effects, and codegen are experimental — not production guarantees.

## [0.1.0] - 2025

### Added

- Initial VS Code extension scaffold: language id `nexus`, TextMate grammar, snippets, basic commands and configuration keys.

[Unreleased]: https://github.com/theworker02/nex-lang/compare/nex-ts-0.4.0...HEAD
[0.2.0]: https://github.com/theworker02/nex-lang/compare/vscode-nexus-0.1.1...vscode-nexus-0.2.0
[0.1.1]: https://github.com/theworker02/nex-lang/releases/tag/vscode-nexus-0.1.1
[0.3.0]: https://github.com/nex-lang/nex-lang/releases/tag/nex-ts-0.3.0
[0.1.0]: https://github.com/nex-lang/nex-lang/releases/tag/vscode-nexus-0.1.0
