# Nexus Design Language

Nexus is not only a general programming language — it is also a **design language**: a declarative surface for themes, typography, spacing, layout, and forms, inspired by how CSS describes presentation while remaining `.nex`.

Design modules author real UI trees that the TypeScript host compiles to **HTML + CSS**, with an optional progressive **nxd.js** kit (search shortcut, copy buttons, mobile/docs drawers).

## Philosophy

| Layer | Role | Analogy |
| --- | --- | --- |
| Theme tokens | Color, type, space, radius, breakpoints, dark mode | CSS custom properties |
| Layout nodes | `stack` / `row` / `grid` / `hero` / `layout` | Flexbox / grid |
| Components | forms, tables, cards, alerts, footer, sidebar | Reusable UI primitives |
| Client kit | `nxd.js` (inlined by `design_document`) | Progressive enhancement |
| Host render | `design_document` / `design_response` | Browser cascade → pixels |

## Quick start

```powershell
cd vscode-nexus
npm run compile
npm run site
# → http://localhost:8090

npm run registry
# → http://localhost:8080  (design-authored home, search, packages, auth, docs, legal)
```

## Stdlib API (`import "design"`)

### Theme

- `theme(tokens)` / `nexus_theme()` — Registry slate/navy/`#1d4ed8` defaults
- Dark mode: `mode` = `system` | `light` | `dark`, plus `dark_*` token overrides
- Breakpoints: `bp_sm` / `bp_md` / `bp_lg`

### Layout & chrome

- `stack` / `row` / `grid` / `hero` / `section` / `shell`
- `layout` — slots: `topbar`, `sidebar`, `body`, `footer` (docs drawer support)
- `brand` / `brand_link`, `topbar` / `topbar_full`, `nav` / `nav_item`
- `footer`, `sidebar`, `card`, `alert` / `banner`

### Forms

- `form`, `field`, `label`, `input`, `search_input`, `textarea`, `select`, `checkbox`, `fieldset`
- `submit_btn` / `submit_btn_secondary` — real `<button type="submit">`
- `button`, `copy_btn` (data-copy → nxd kit), `hidden_input`

### Media & tables

- `image` / `image_sized`, `figure`
- `table`, `table_row` / `table_head`, `table_cell` / `table_header_cell`

### Pages

- `page`, `page_full`, `page_layout`
- Default `kit: true` injects nxd.js into the document

## Host builtins

| Builtin | Result |
| --- | --- |
| `design_document(page)` | Full HTML document (+ nxd kit unless `kit: false`) |
| `design_response(page [, status])` | HTTP HTML response |
| `design_render(node [, theme])` | HTML fragment |
| `design_css(theme)` | CSS text |
| `html_doc(html [, status])` | Raw HTML response |

Also: `GET /static/js/nxd.js` serves the same kit as a standalone script.

## Registry pages (design-authored)

Major user-facing routes in `nex-registry` now return `design_response(...)`:

| Route | Module |
| --- | --- |
| `/`, `/search`, `/packages`, `/packages/{name}` | `app/design/registry_pages.nex` via `web.nex` |
| `/docs`, `/docs/{page}` | docs shell + sidebar |
| `/login`, `/register`, `/settings` | `auth.nex` |
| `/legal/*` | `legal.nex` |
| `/design`, `/design/guide` | original design showcase |

Remaining HTML templates cover admin/org edge flows, 2FA, forgot/reset password, discover/keywords, report, etc.

## Database modes (TS host)

| Mode | When | Behavior |
| --- | --- | --- |
| **Memory** | `DATABASE_URL` unset | Demo store seeded from `storage/` |
| **Postgres** | `DATABASE_URL` set | Real `pg` driver via sync worker; migrations from `MIGRATIONS_DIR`; users/sessions/packages/auth tokens |

```powershell
# Memory (default)
npm run registry

# Postgres
$env:DATABASE_URL = "postgres://user:pass@localhost:5432/nexus"
$env:MIGRATIONS_DIR = "..\..\nex-registry\migrations"
npm run registry
```

Cookie `Secure` is set automatically when `BASE_URL` is `https://`, or when `COOKIE_SECURE=1`.

## Related docs

- [Website apps](../website.md)
- [Guide](guide.md)
- [Builtins](../builtins.md)
