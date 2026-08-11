# Changelog

Notable changes across the `nex-lang` monorepo.

Detailed TypeScript toolchain history: [vscode-nexus/CHANGELOG.md](vscode-nexus/CHANGELOG.md).

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- **Governance / trust docs**: `SECURITY.md`, `PRIVACY.md`, `TERMS.md`, `CODE_OF_CONDUCT.md`, `CONTRIBUTING.md`, `SUPPORT.md` — linked from README; site routes `/privacy`, `/security`, `/terms`, `/conduct`.
- **Self-host site readiness**: language homepage via design language (`npm run site` / `npm run build:site`), GitHub Pages + CI/Open VSX workflows, funding file — publishable without `nex-registry`.
- Brand mark (ribbon N) under `assets/` and `vscode-nexus/media/`.
- Primary documentation under `vscode-nexus/docs/`.
- Design language + standalone site demos.

### Changed

- TypeScript toolchain is the supported path; optional local registry sibling is not part of the published repo.
- Repo remotes / extension publisher aligned to `theworker02`; extension display name **Nex LSP** (`nex-lsp`).

### GitHub release notes snippet

> Nexus language toolchain is now self-hosting its marketing/docs site (live + static GitHub Pages), with Open VSX packaging/publish workflows and clearer install docs. Clone, `cd vscode-nexus && npm install && npm run compile && npm run site`.

## [0.3.0] - 2026-08-09

### Added

- TypeScript language core: eval, bytecode VM, modules, stdlib, REPL, tests, selfhost (`.nex` lexer/parser/evaluator).
- TS web host capable of serving `nex-registry` demos with in-memory DB (`npm run registry`).
- Go CLI language pack (tree-walk + VM + modules + host) retained for Postgres and `try`.

### Changed

- Active self-host / website story: **TypeScript first**, Go optional.
