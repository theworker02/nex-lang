# Language website (self-host + GitHub Pages)

Nexus ships a **Rust-lang.org–style** language homepage authored in the design language under `examples/site/`. It does **not** require the private `nex-registry` sibling.

## Live (TypeScript host)

```powershell
cd vscode-nexus
npm run compile
npm run site
# → http://localhost:8090
```

Routes: `/`, `/docs`, `/install`, `/design`, `/guide`, `/privacy`, `/security`, `/terms`, `/conduct`, `/healthz`. Logos under `examples/site/web/static/img/`.

Trust / legal pages mirror repo root `PRIVACY.md`, `SECURITY.md`, `TERMS.md`, and `CODE_OF_CONDUCT.md` (maintained by theworker02 / nex-lang project maintainers).

## Static export

```powershell
cd vscode-nexus
npm run build:site
# writes ../site/  (gitignored; used by CI / Pages)
```

Optional `NEX_SITE_BASE=/nex-lang` prefixes absolute `/…` links for GitHub project Pages.

## GitHub Pages

Workflow: [`.github/workflows/pages.yml`](../../.github/workflows/pages.yml) — on push to `main`, builds the static site and deploys.

URL: https://theworker02.github.io/nex-lang/

Enable Pages in the GitHub repo: **Settings → Pages → Source: GitHub Actions**.

## Optional local registry

`npm run registry` still works when a local `nex-registry` checkout exists beside this repo. That app is intentionally **not** published with `nex-lang`.
