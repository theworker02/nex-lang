# Design language guide

Practical walkthrough. For the full API see [README.md](README.md).

## 1. Tokens

```nex
import "design";

theme app {
  ink = "#0f172a"
  paper = "#f1f5f9"
  accent = "#1d4ed8"
  space_4 = "1rem"
  space_6 = "1.5rem"
  font_display = "\"Bricolage Grotesque\", sans-serif"
  font_body = "\"Figtree\", sans-serif"
}
```

Or start from `nexus_theme()` and override via `merge` if you prefer functions over sugar.

## 2. Compose a page

```nex
view landing = page_full(
  "Title",
  "Description for meta tags",
  app,
  topbar(
    brand_link("Nexus", "Design", "/", "/static/img/logo.png"),
    nav([nav_item("/", "Home", true), nav_item("/docs", "Docs", false)])
  ),
  [
    hero([
      kicker("Product"),
      headline("One composition"),
      lead("Brand first. One headline. One supporting sentence. One CTA group."),
      row(style { gap = "space_3" }, [
        link_btn("/start", "Start"),
        link_btn_secondary("/docs", "Docs")
      ])
    ]),
    section(style { id = "next" }, [
      stack(style { gap = "space_4" }, [
        body_text("Secondary sections do one job each."),
        codeblock("nex", "return design_response(landing);")
      ])
    ])
  ]
);
```

## 3. Serve it

```nex
http_get("/", fn(req) {
  return design_response(landing);
});
```

Point `NEX_WEB_DIR` at a tree that has `static/img/logo.png` (or `logo.svg`) if you use the Registry mark (the `npm run site` script does this automatically when `nex-registry` is a sibling). The mark file is the **ribbon N only** — put names like “Nexus” / “Package Registry” in `brand_link` text arguments, not inside the image.

## 4. Forms and the client kit

```nex
form(style { action = "/login", method = "post" }, [
  field("Email", input({ "name": "email", "type": "email", "required": true })),
  field("Password", input({ "name": "password", "type": "password", "required": true })),
  submit_btn("Log in")
])
```

`design_document` inlines **nxd.js** by default (`kit: true`): `/` focus via Ctrl/Cmd+K, `[data-copy]` buttons, and mobile/docs drawers (`#nav-toggle`, `#mobile-nav`, `#sidebar`).

Dark mode: set theme token `mode` to `system` (default), `light`, or `dark`, and optional `dark_*` colors.

## 5. How the registry designs itself

1. `app/design/theme.nex` — token theme (incl. dark + breakpoints)  
2. `app/design/layout.nex` — shared topbar/footer  
3. `app/design/registry_pages.nex` — home, search, packages, auth, docs, legal  
4. `app/design/pages.nex` + `routes.nex` — `/design` showcase  
5. `app/main.nex` imports design + web/auth/legal routes  

Visit `http://localhost:8080/` after `npm run registry` — the home page is design-authored.