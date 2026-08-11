# Contributing to Nexus

Thanks for your interest in contributing to **nex-lang**. Maintainers: **theworker02 / nex-lang project maintainers**.

## Before you start

- Read the [Code of Conduct](CODE_OF_CONDUCT.md).
- Prefer changes in **`vscode-nexus/`** — the TypeScript toolchain is the supported path.
- Keep docs honest: document implemented behavior; call out subsets, stubs, and Go-only features.
- Do **not** commit secrets (`.env`, tokens, private keys).
- Do **not** publish or fold the private `nex-registry` sibling into this repository.

## Development setup

```powershell
cd vscode-nexus
npm install
npm run compile
npm run smoke
npm run test:nex
```

Useful commands:

| Command | Purpose |
| --- | --- |
| `npm run compile` | Build TypeScript → `out/` |
| `npm run smoke` | Quick sanity checks |
| `npm run test:nex` | Language tests |
| `npm run site` | Live language homepage → http://localhost:8090 |
| `npm run build:site` | Static export → `../site/` (GitHub Pages) |
| `npm run package` | Build Nex LSP VSIX |

CLI after compile: `node out/cli.js help`.

## What to work on

Good contribution areas:

- Language features with tests under `vscode-nexus/tests/`
- Docs under `vscode-nexus/docs/` and honesty fixes in `docs/spec.md`
- Design-language site pages under `vscode-nexus/examples/site/`
- Editor / LSP improvements in the extension package
- Bug fixes with a clear reproduction

Ask in an issue first for large architecture changes.

## Pull requests

1. Fork and branch from `main`.
2. Keep PRs focused and reviewable.
3. Run `npm run compile`, `npm run smoke`, and `npm run test:nex` (and `npm run build:site` if you touch the site).
4. Update [CHANGELOG.md](CHANGELOG.md) / [vscode-nexus/CHANGELOG.md](vscode-nexus/CHANGELOG.md) when user-facing behavior changes.
5. Language/grammar changes: update language docs / `docs/spec.md` when behavior ships.
6. Brand: use **mark-only** assets from `assets/` / `media/`; never bake “NEXUS” into logo files.
7. Go changes (`cmd/nex`, `pkg/*`): note TS vs Go divergences (`try`, Postgres host, etc.).

## Security

Report vulnerabilities privately — see [SECURITY.md](SECURITY.md). Do not open public issues for sensitive reports.

## Support vs bugs

- How-to questions and discussion: see [SUPPORT.md](SUPPORT.md).
- Bugs and feature requests: GitHub Issues on [theworker02/nex-lang](https://github.com/theworker02/nex-lang/issues).

## License

By contributing, you agree that your contributions are licensed under the same [MIT License](LICENSE) as the project.

## Open VSX publish (maintainers)

Publishing **Nex LSP** to Open VSX is optional and runs via [`.github/workflows/openvsx.yml`](.github/workflows/openvsx.yml). Maintainers configure repository secret `OVSX_PAT` (token from [open-vsx.org/user-settings/tokens](https://open-vsx.org/user-settings/tokens)). Contributors do not need this secret to develop or open PRs — CI packages the VSIX without publishing when the secret is absent.
