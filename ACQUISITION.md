# Acquisition Brief â€” Run programs

**Date:** 2026-09-22  
**Repository:** https://github.com/theworker02/nex-lang  
**Default branch:** `main`  
**Primary language:** TypeScript  
**Status:** Diligence briefing only. **No acquisition has occurred** by virtue of this file.  
**License:** Proprietary â€” sale, written commercial license, or completed asset transfer required (see root `LICENSE`).  
**Valuation:** Not stated.  
**Contact:** GitHub [@theworker02](https://github.com/theworker02) Â· [thanks.dev/u/gh/theworker02](https://thanks.dev/u/gh/theworker02)

> Cloning or forking this repository does **not** grant production, redistribution, SaaS, OEM, or commercial rights.

---

## 1. Executive thesis

<img src="assets/logo.png" alt="Nexus" width="160" height="160"> A gradually typed, expression-oriented language with a TypeScript-first toolchain,<br> bytecode VM, self-hosted <code>.nex</code> pipeline, design language, and upcoming package registry.

**Why a buyer cares:** Run programs packages transferable product IP â€” source, docs, in-repo brand assets, and a diligence room under `docs/acquisition/` â€” under a clear proprietary posture so diligence can proceed without mistaking the repo for open source.

---

## 2. Product snapshot

| Item | Detail |
|------|--------|
| Product | Run programs |
| Repo | `theworker02/nex-lang` |
| Language | TypeScript |
| Open source? | **No** â€” proprietary |
| Rightsholder | theworker02 |
| Diligence pack | `docs/acquisition/` |

### Capability highlights (from current materials)

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

---

## 3. Problem / opportunity

Teams evaluating Run programs typically need either (a) a commercial right to run or embed it, or (b) outright ownership of the Product IP for strategic build-out. Public GitHub visibility without a proprietary license creates false assumptions about free production use. This brief and the linked data room make the commercial path explicit.

---

## 4. What ships today

Honest maturity: treat repository contents, README claims, tests, and release tags as the source of truth. Do not assume production customers, ARR, filed patents, or SLAs unless separately evidenced in diligence.

Typical transferable surfaces:

- Source tree and build/test scripts present in-repo
- Documentation and design notes
- Acquisition / diligence markdown under `docs/acquisition/`
- Branding assets committed to the repository (if any)

---

## 5. Demo / evaluation path (buyer)

Minimal path (no secrets required unless README says otherwise):

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

Extended evaluation: `docs/acquisition/BUYER_EVALUATION.md`. Written NDA / evaluation grants may be required for private materials.

---

## 6. What a transaction typically includes

Subject to definitive schedules:

| Included (typical) | Excluded (typical) |
|--------------------|--------------------|
| Repo materials + asserted original IP | Seller personal accounts / unrelated repos |
| Docs + diligence room at closing | Third-party dependency source under separate licenses |
| In-repo brand marks as assigned | Secrets without rotation plan |
| Know-how captured in docs | Fabricated revenue, user, or adoption metrics |

---

## 7. Suggested deal structures

| Structure | When it fits |
|-----------|--------------|
| Non-exclusive commercial license | Deploy/run under seat or environment terms |
| Exclusive field-of-use license | Buyer wants exclusivity; seller may retain entity |
| Asset / IP assignment | Buyer wants ownership of Materials outright |
| OEM / redistribution | Separate agreement â€” not implied here |

Commercial terms (price, earnouts, escrow) are negotiated under NDA with counsel.

---

## 8. Buyer diligence checklist

- [ ] Confirm Rightsholder identity and authority to sell/license
- [ ] Inventory Materials (`docs/acquisition/ASSET_INVENTORY.md`)
- [ ] Review IP posture (`IP_PROVENANCE.md`) and dependencies (`DEPENDENCY_INVENTORY.md`)
- [ ] Run evaluation script (`BUYER_EVALUATION.md`)
- [ ] Review risks (`RISK_REGISTER.md`)
- [ ] Agree transfer scope (`TRANSFER_MANIFEST.md`) and handoff (`HANDOFF_CHECKLIST.md`)
- [ ] Supersede root `LICENSE` at closing via definitive agreement

---

## 9. Related documents

| Document | Purpose |
|----------|---------|
| `LICENSE` | Proprietary â€” no default grant |
| `docs/acquisition/README.md` | Data-room index |
| `docs/acquisition/EXECUTIVE_SUMMARY.md` | One-page thesis |
| `README.md` | Product overview |
| `SECURITY.md` | Vulnerability reporting |
| `COMMERCIAL.md` | Licensing contact path |
| `.github/FUNDING.yml` | Sponsors / thanks.dev |

---

## 10. Disclaimer

This package is informational and **does not** create a binding offer, grant of rights, or investment advice. Engage counsel for any transaction.

---

*Document version: 2.0.0 / 2026-09-22 Â· Classification: acquisition briefing*
