# Asset inventory â€” Run programs

## Repository surfaces

| Asset | Location / notes |
|-------|------------------|
| Source tree | Repository root / language packages |
| Tests | `test/`, `tests/`, CI workflows if present |
| Docs | `README.md`, `docs/` |
| Diligence room | `docs/acquisition/` |
| License / notices | `LICENSE`, transition notices if present |
| Funding | `.github/FUNDING.yml` |
| CI | `.github/workflows/` if present |
| Branding | logos/assets folders if present |

## Capability highlights

- **Primary runtime:** TypeScript host under [`vscode-nexus/`](vscode-nexus/) (tree-walk evaluator + optional bytecode VM).
- **Editor:** **Nex LSP** Ã¢â‚¬â€ install from [Open VSX](https://open-vsx.org/extension/theworker02/nex-lsp) (`theworker02.nex-lsp`) for VS Code / VSCodium; also built from this package.
- **Self-hosting:** Lexer / parser / evaluator written in `.nex` under [`vscode-nexus/selfhost/`](vscode-nexus/selfhost/), loaded by the TS host.
- **Design language:** Declarative UI themes + layout in `.nex` Ã¢â€ â€™ real HTML/CSS via host builtins.
- **Language site:** Design-authored homepage + docs landing via `npm run site` / `npm run build:site` (GitHub Pages).
- **Packages:** `nexus.toml` + publish/install client; optional **local** sibling `nex-registry` for package-hub demos (not published with this repo).
- **Legacy Go CLI:** Still in this monorepo (`cmd/nex`, `pkg/*`) for some Go-only features (notably `try`).
- `let`, functions / closures, `if` / `while` / `for`, `break` / `continue`
- Gradual type annotations
- Arrays, hashes, indexing / members
- `struct` / `enum`, `match` (`->` / `=>`)
- Pipes `|>`, Results `ok` / `err` / `unwrap` (TS path; `try` early-return is **Go-only**)

## Usually excluded

Seller personal accounts, unrelated repos, and unreissued registry tokens â€” unless listed in the definitive agreement.

*Updated: 2026-09-22*
