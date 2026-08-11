# Changelog

All notable changes to the Nexus TypeScript toolchain (`vscode-nexus`) are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to follow [Semantic Versioning](https://semver.org/) for the language CLI (`nex-ts`) and extension package where practical.

## [Unreleased]

### Added

- **Trust pages on language site**: `/privacy`, `/security`, `/terms`, `/conduct` (live + static export); footer links to governance docs.
- **Self-host language site**: design-authored homepage under `examples/site/` (`npm run site`) with bundled logos — no private registry required.
- **Static site export** (`npm run build:site`) and GitHub Pages workflow (`.github/workflows/pages.yml`) → https://theworker02.github.io/nex-lang/
- **Open VSX publish** workflow (`.github/workflows/openvsx.yml`) using `OVSX_PAT`; CI packages VSIX without requiring the secret.
- **Funding** file (`.github/FUNDING.yml`) for GitHub Sponsors (`theworker02`).
- CI workflow compiling, smoking, testing, building the static site, and packaging the extension.
- Docs: `docs/site.md`; honest Postgres/`DATABASE_URL` notes for the TS host.

### Changed

- Extension branded as **Nex LSP** (`theworker02.nex-lsp`, icon: ribbon N `media/logo-256.png`) for Open VSX / marketplace.
- Extension `publisher` set to `theworker02` for Open VSX alignment.
- Root `.gitignore` covers `node_modules`, `out/`, `.env`, `site/`, VSIX artifacts.
- Registry documented as optional local sibling only (not published with nex-lang).

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

[Unreleased]: https://github.com/nex-lang/nex-lang/compare/nex-ts-0.3.0...HEAD
[0.3.0]: https://github.com/nex-lang/nex-lang/releases/tag/nex-ts-0.3.0
[0.1.0]: https://github.com/nex-lang/nex-lang/releases/tag/vscode-nexus-0.1.0
