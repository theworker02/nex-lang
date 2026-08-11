# Changelog

Notable changes across the `nex-lang` monorepo.

Detailed TypeScript toolchain history: [vscode-nexus/CHANGELOG.md](vscode-nexus/CHANGELOG.md).

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.4.0] - 2026-08-11

### Added

- Shared static checker (`checkProgram`) for undefined names, call arity mismatches, and unused locals.
- `nex-ts check <file.nex>` CLI command for static analysis without execution.
- VS Code diagnostics integration for static checker warnings and errors.

## [0.3.0] - 2026-08-09

### Added

- TypeScript language core: eval, bytecode VM, modules, stdlib, REPL, tests, selfhost (`.nex` lexer/parser/evaluator).
- TS web host capable of serving `nex-registry` demos with in-memory DB (`npm run registry`).
- Go CLI language pack (tree-walk + VM + modules + host) retained for Postgres and `try`.

### Changed

- Active self-host / website story: **TypeScript first**, Go optional.
