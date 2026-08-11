/**
 * Nexus Design Language renderer.
 * Compiles design trees (hashes tagged with `_design`) into HTML + CSS.
 * Progressive enhancement kit (`nxd.js`) is inlined by design documents.
 */

export type DesignData = unknown;

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function asString(v: unknown, fallback = ""): string {
  if (typeof v === "string") {
    return v;
  }
  if (typeof v === "number" || typeof v === "boolean") {
    return String(v);
  }
  return fallback;
}

function asArray(v: unknown): unknown[] {
  return Array.isArray(v) ? v : [];
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

export interface ThemeTokens {
  ink: string;
  inkSoft: string;
  paper: string;
  mist: string;
  surface: string;
  line: string;
  accent: string;
  accentDeep: string;
  accentSoft: string;
  onAccent: string;
  fontDisplay: string;
  fontBody: string;
  fontMono: string;
  radius: string;
  max: string;
  space1: string;
  space2: string;
  space3: string;
  space4: string;
  space6: string;
  space8: string;
  space12: string;
  ease: string;
  bpSm: string;
  bpMd: string;
  bpLg: string;
  mode: string;
  darkInk: string;
  darkInkSoft: string;
  darkPaper: string;
  darkMist: string;
  darkSurface: string;
  darkLine: string;
  darkAccent: string;
  darkAccentDeep: string;
  darkAccentSoft: string;
  darkOnAccent: string;
}

export const NEXUS_DEFAULT_THEME: ThemeTokens = {
  ink: "#0f172a",
  inkSoft: "#475569",
  paper: "#f1f5f9",
  mist: "#e2e8f0",
  surface: "#ffffff",
  line: "rgba(15, 23, 42, 0.1)",
  accent: "#1d4ed8",
  accentDeep: "#1e40af",
  accentSoft: "rgba(29, 78, 216, 0.1)",
  onAccent: "#ffffff",
  fontDisplay: '"Bricolage Grotesque", "Segoe UI", sans-serif',
  fontBody: '"Figtree", "Segoe UI", sans-serif',
  fontMono: '"IBM Plex Mono", "Consolas", monospace',
  radius: "0.35rem",
  max: "72rem",
  space1: "0.25rem",
  space2: "0.5rem",
  space3: "0.75rem",
  space4: "1rem",
  space6: "1.5rem",
  space8: "2rem",
  space12: "3rem",
  ease: "cubic-bezier(0.22, 1, 0.36, 1)",
  bpSm: "640px",
  bpMd: "800px",
  bpLg: "1100px",
  mode: "system",
  darkInk: "#e8eef7",
  darkInkSoft: "#94a3b8",
  darkPaper: "#0b1220",
  darkMist: "#1e293b",
  darkSurface: "#111827",
  darkLine: "rgba(232, 238, 247, 0.12)",
  darkAccent: "#3b82f6",
  darkAccentDeep: "#60a5fa",
  darkAccentSoft: "rgba(59, 130, 246, 0.16)",
  darkOnAccent: "#0b1220",
};

const TOKEN_ALIASES: Record<string, keyof ThemeTokens> = {
  ink: "ink",
  ink_soft: "inkSoft",
  "ink-soft": "inkSoft",
  paper: "paper",
  mist: "mist",
  surface: "surface",
  line: "line",
  accent: "accent",
  accent_deep: "accentDeep",
  "accent-deep": "accentDeep",
  accent_soft: "accentSoft",
  "accent-soft": "accentSoft",
  on_accent: "onAccent",
  "on-accent": "onAccent",
  font_display: "fontDisplay",
  "font-display": "fontDisplay",
  font_body: "fontBody",
  "font-body": "fontBody",
  font_mono: "fontMono",
  "font-mono": "fontMono",
  radius: "radius",
  max: "max",
  space_1: "space1",
  space1: "space1",
  space_2: "space2",
  space2: "space2",
  space_3: "space3",
  space3: "space3",
  space_4: "space4",
  space4: "space4",
  space_6: "space6",
  space6: "space6",
  space_8: "space8",
  space8: "space8",
  space_12: "space12",
  space12: "space12",
  ease: "ease",
  bp_sm: "bpSm",
  "bp-sm": "bpSm",
  bp_md: "bpMd",
  "bp-md": "bpMd",
  bp_lg: "bpLg",
  "bp-lg": "bpLg",
  mode: "mode",
  dark_ink: "darkInk",
  "dark-ink": "darkInk",
  dark_ink_soft: "darkInkSoft",
  "dark-ink-soft": "darkInkSoft",
  dark_paper: "darkPaper",
  "dark-paper": "darkPaper",
  dark_mist: "darkMist",
  "dark-mist": "darkMist",
  dark_surface: "darkSurface",
  "dark-surface": "darkSurface",
  dark_line: "darkLine",
  "dark-line": "darkLine",
  dark_accent: "darkAccent",
  "dark-accent": "darkAccent",
  dark_accent_deep: "darkAccentDeep",
  "dark-accent-deep": "darkAccentDeep",
  dark_accent_soft: "darkAccentSoft",
  "dark-accent-soft": "darkAccentSoft",
  dark_on_accent: "darkOnAccent",
  "dark-on-accent": "darkOnAccent",
};

export function themeFromData(data: DesignData): ThemeTokens {
  const theme = { ...NEXUS_DEFAULT_THEME };
  if (!isRecord(data)) {
    return theme;
  }
  const tokens = isRecord(data.tokens)
    ? data.tokens
    : data._design === "theme"
      ? data
      : data;
  const source = isRecord(tokens) && isRecord(tokens.tokens) ? tokens.tokens : tokens;
  for (const [k, v] of Object.entries(source)) {
    if (k === "_design" || k === "tokens" || k === "name") {
      continue;
    }
    const key = TOKEN_ALIASES[k];
    if (key && typeof v === "string") {
      theme[key] = v;
    }
  }
  return theme;
}

function resolveSpace(theme: ThemeTokens, value: string): string {
  const key = TOKEN_ALIASES[value];
  if (key && key.startsWith("space")) {
    return theme[key];
  }
  const spaced = TOKEN_ALIASES[`space_${value}`] ?? TOKEN_ALIASES[`space${value}`];
  if (spaced) {
    return theme[spaced];
  }
  return value;
}

function classNames(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

function prop(node: Record<string, unknown>, key: string, fallback = ""): string {
  return asString(node[key], fallback);
}

function childrenOf(node: Record<string, unknown>): unknown[] {
  if (Array.isArray(node.children)) {
    return node.children;
  }
  if (Array.isArray(node.body)) {
    return node.body;
  }
  if (node.child !== undefined) {
    return [node.child];
  }
  return [];
}

function boolAttr(node: Record<string, unknown>, key: string): boolean {
  const v = node[key];
  return v === true || v === "true" || v === 1 || v === "1" || v === "on";
}

function renderChildren(node: Record<string, unknown>, theme: ThemeTokens): string {
  return childrenOf(node)
    .map((c) => renderNode(c, theme))
    .join("\n");
}

function btnClass(variant: string): string {
  return classNames(
    variant === "text" ? "nxd-link" : "nxd-btn",
    variant === "primary" && "nxd-btn-primary",
    variant === "secondary" && "nxd-btn-secondary",
    variant === "ghost" && "nxd-btn-ghost",
  );
}

function renderNode(node: DesignData, theme: ThemeTokens): string {
  if (node === null || node === undefined) {
    return "";
  }
  if (typeof node === "string" || typeof node === "number" || typeof node === "boolean") {
    return `<span class="nxd-text nxd-text-body">${escapeHtml(String(node))}</span>`;
  }
  if (Array.isArray(node)) {
    return node.map((n) => renderNode(n, theme)).join("\n");
  }
  if (!isRecord(node)) {
    return "";
  }

  const kind = asString(node._design || node.kind || node.type, "box");

  switch (kind) {
    case "theme":
      return "";
    case "page":
      return renderNode(node.body ?? node.content ?? node.children, theme);
    case "stack": {
      const gap = resolveSpace(theme, prop(node, "gap", "space_4"));
      const pad = prop(node, "pad")
        ? resolveSpace(theme, prop(node, "pad"))
        : undefined;
      const align = prop(node, "align", "stretch");
      const style = `gap:${gap};align-items:${align}${pad ? `;padding:${pad}` : ""}`;
      return `<div class="nxd-stack" style="${style}">${renderChildren(node, theme)}</div>`;
    }
    case "row": {
      const gap = resolveSpace(theme, prop(node, "gap", "space_3"));
      const pad = prop(node, "pad")
        ? resolveSpace(theme, prop(node, "pad"))
        : undefined;
      const align = prop(node, "align", "center");
      const wrap = node.wrap === false ? "nowrap" : "wrap";
      const style = `gap:${gap};align-items:${align};flex-wrap:${wrap}${
        pad ? `;padding:${pad}` : ""
      }`;
      return `<div class="nxd-row" style="${style}">${renderChildren(node, theme)}</div>`;
    }
    case "grid": {
      const cols = asString(node.columns ?? node.cols, "2");
      const gap = resolveSpace(theme, prop(node, "gap", "space_4"));
      const style = `grid-template-columns:repeat(${escapeHtml(cols)},minmax(0,1fr));gap:${gap}`;
      return `<div class="nxd-grid" style="${style}">${renderChildren(node, theme)}</div>`;
    }
    case "box":
    case "section": {
      const id = prop(node, "id");
      const pad = prop(node, "pad")
        ? resolveSpace(theme, prop(node, "pad"))
        : undefined;
      const cls = classNames("nxd-section", prop(node, "class"));
      const style = pad ? `padding:${pad}` : "";
      return `<section class="${cls}"${id ? ` id="${escapeHtml(id)}"` : ""}${
        style ? ` style="${style}"` : ""
      }>${renderChildren(node, theme)}</section>`;
    }
    case "hero": {
      return `<section class="nxd-hero">${renderChildren(node, theme)}</section>`;
    }
    case "brand": {
      const name = prop(node, "name", "Nexus");
      const tag = prop(node, "tag", "");
      const href = prop(node, "href", "/");
      const mark = prop(node, "mark", "/static/img/logo.png");
      return `<a class="nxd-brand" href="${escapeHtml(href)}" aria-label="${escapeHtml(
        name,
      )}">
  <img class="nxd-brand-mark" src="${escapeHtml(mark)}" width="40" height="40" alt="" decoding="async">
  <span class="nxd-brand-text">
    <span class="nxd-brand-name">${escapeHtml(name)}</span>
    ${tag ? `<span class="nxd-brand-tag">${escapeHtml(tag)}</span>` : ""}
  </span>
</a>`;
    }
    case "text":
    case "headline":
    case "lead":
    case "kicker":
    case "code": {
      const variant =
        kind === "text"
          ? prop(node, "variant", "body")
          : kind === "code"
            ? "code"
            : kind;
      const content = prop(node, "content", prop(node, "text", ""));
      const tag =
        variant === "headline"
          ? "h1"
          : variant === "lead"
            ? "p"
            : variant === "kicker"
              ? "p"
              : variant === "code"
                ? "code"
                : "p";
      const cls = classNames(`nxd-text`, `nxd-text-${variant}`);
      const inner =
        variant === "headline" && content.includes("*")
          ? content
              .split(/(\*[^*]+\*)/g)
              .map((part) =>
                part.startsWith("*") && part.endsWith("*")
                  ? `<em>${escapeHtml(part.slice(1, -1))}</em>`
                  : escapeHtml(part),
              )
              .join("")
          : escapeHtml(content);
      return `<${tag} class="${cls}">${inner}</${tag}>`;
    }
    case "link": {
      const href = prop(node, "href", "#");
      const label = prop(node, "label", prop(node, "text", "Link"));
      const variant = prop(node, "variant", "text");
      return `<a class="${btnClass(variant)}" href="${escapeHtml(href)}">${escapeHtml(label)}</a>`;
    }
    case "button": {
      const label = prop(node, "label", prop(node, "text", "Button"));
      const variant = prop(node, "variant", "primary");
      const type = prop(node, "type", "button");
      const id = prop(node, "id", "");
      const copy = node.copy !== undefined && node.copy !== null && node.copy !== false
        ? ` data-copy="${escapeHtml(asString(node.copy))}"`
        : "";
      return `<button type="${escapeHtml(type)}" class="${btnClass(variant)}"${
        id ? ` id="${escapeHtml(id)}"` : ""
      }${copy}>${escapeHtml(label)}</button>`;
    }
    case "submit": {
      const label = prop(node, "label", prop(node, "text", "Submit"));
      const variant = prop(node, "variant", "primary");
      return `<button type="submit" class="${btnClass(variant)}">${escapeHtml(label)}</button>`;
    }
    case "spacer": {
      const size = resolveSpace(theme, prop(node, "size", "space_6"));
      return `<div class="nxd-spacer" style="height:${size}" aria-hidden="true"></div>`;
    }
    case "rule":
      return `<hr class="nxd-rule">`;
    case "codeblock": {
      const lang = prop(node, "lang", "nex");
      const content = prop(node, "content", prop(node, "code", ""));
      return `<pre class="nxd-codeblock" data-lang="${escapeHtml(
        lang,
      )}"><code>${escapeHtml(content)}</code></pre>`;
    }
    case "list": {
      const items = asArray(node.items).length
        ? asArray(node.items)
        : childrenOf(node);
      const ordered = node.ordered === true;
      const tag = ordered ? "ol" : "ul";
      return `<${tag} class="nxd-list">${items
        .map((item) => {
          if (isRecord(item) && item._design) {
            return `<li>${renderNode(item, theme)}</li>`;
          }
          return `<li>${escapeHtml(asString(item))}</li>`;
        })
        .join("\n")}</${tag}>`;
    }
    case "nav": {
      const items = asArray(node.items);
      const id = prop(node, "id", "nxd-nav");
      return `<nav class="nxd-nav" id="${escapeHtml(id)}" aria-label="${escapeHtml(
        prop(node, "label", "Primary"),
      )}">${items
        .map((item) => {
          if (!isRecord(item)) {
            return "";
          }
          const href = asString(item.href, "#");
          const label = asString(item.label ?? item.text, href);
          const current = item.current === true;
          return `<a class="nxd-nav-link"${
            current ? ' aria-current="page"' : ""
          } href="${escapeHtml(href)}">${escapeHtml(label)}</a>`;
        })
        .join("\n")}</nav>`;
    }
    case "topbar": {
      const brand = node.brand;
      const nav = node.nav;
      const drawer = node.drawer !== false;
      const toggle = drawer
        ? `<button type="button" class="nxd-nav-toggle" id="nav-toggle" aria-expanded="false" aria-controls="mobile-nav" aria-label="Open menu">
  <span></span><span></span><span></span>
</button>`
        : "";
      const mobile =
        drawer && isRecord(nav)
          ? `<div class="nxd-mobile-nav" id="mobile-nav" hidden>
  ${renderNode({ ...nav, id: "mobile-nav-links" }, theme)}
  <button type="button" class="nxd-nav-close" id="nav-close" aria-label="Close menu">Close</button>
</div>
<div class="nxd-nav-backdrop" id="nav-backdrop" hidden></div>`
          : "";
      return `<header class="nxd-topbar">
  <div class="nxd-topbar-inner">
    ${renderNode(brand, theme)}
    <div class="nxd-topbar-end">
      ${renderNode(nav, theme)}
      ${toggle}
    </div>
  </div>
  ${mobile}
</header>`;
    }
    case "shell": {
      return `<div class="nxd-shell">${renderChildren(node, theme)}</div>`;
    }
    case "layout": {
      const top = node.topbar ? renderNode(node.topbar, theme) : "";
      const side = node.sidebar ? renderNode(node.sidebar, theme) : "";
      const foot = node.footer ? renderNode(node.footer, theme) : "";
      const body = renderNode(node.body ?? node.content ?? node.children ?? [], theme);
      const docs = boolAttr(node, "docs") || boolAttr(node, "docs_mode");
      const cls = classNames("nxd-layout", docs && "nxd-layout-docs", side && "nxd-layout-aside");
      return `${top}
<div class="${cls}">
  ${side}
  <div class="nxd-layout-main">${body}</div>
</div>
${foot}`;
    }
    case "form": {
      const action = prop(node, "action", "");
      const method = prop(node, "method", "post");
      const role = prop(node, "role");
      const cls = classNames("nxd-form", prop(node, "class"));
      const id = prop(node, "id");
      return `<form class="${cls}"${action ? ` action="${escapeHtml(action)}"` : ""} method="${escapeHtml(method)}"${
        role ? ` role="${escapeHtml(role)}"` : ""
      }${id ? ` id="${escapeHtml(id)}"` : ""}>${renderChildren(node, theme)}</form>`;
    }
    case "label": {
      const forId = prop(node, "for", prop(node, "htmlFor", ""));
      const text = prop(node, "text", prop(node, "label", ""));
      const cls = classNames("nxd-label", prop(node, "class"));
      const inner = childrenOf(node).length
        ? renderChildren(node, theme)
        : escapeHtml(text);
      return `<label class="${cls}"${forId ? ` for="${escapeHtml(forId)}"` : ""}>${inner}</label>`;
    }
    case "field": {
      const labelText = prop(node, "label", "");
      const control = node.control ?? node.input;
      return `<label class="nxd-field">
  <span class="nxd-field-label">${escapeHtml(labelText)}</span>
  ${renderNode(control, theme)}
</label>`;
    }
    case "input": {
      const type = prop(node, "type", "text");
      const name = prop(node, "name", "");
      const value = prop(node, "value", "");
      const placeholder = prop(node, "placeholder", "");
      const id = prop(node, "id", name);
      const autocomplete = prop(node, "autocomplete", "");
      const required = boolAttr(node, "required") ? " required" : "";
      const autofocus = boolAttr(node, "autofocus") ? " autofocus" : "";
      const search = node.search === true || type === "search" ? " data-search-input" : "";
      const cls = classNames("nxd-input", prop(node, "class"));
      if (type === "hidden") {
        return `<input type="hidden" name="${escapeHtml(name)}" value="${escapeHtml(value)}">`;
      }
      return `<input class="${cls}" type="${escapeHtml(type)}" name="${escapeHtml(name)}" id="${escapeHtml(id)}" value="${escapeHtml(value)}"${
        placeholder ? ` placeholder="${escapeHtml(placeholder)}"` : ""
      }${autocomplete ? ` autocomplete="${escapeHtml(autocomplete)}"` : ""}${required}${autofocus}${search}>`;
    }
    case "textarea": {
      const name = prop(node, "name", "");
      const value = prop(node, "value", prop(node, "content", ""));
      const id = prop(node, "id", name);
      const rows = prop(node, "rows", "4");
      const placeholder = prop(node, "placeholder", "");
      const required = boolAttr(node, "required") ? " required" : "";
      return `<textarea class="nxd-textarea" name="${escapeHtml(name)}" id="${escapeHtml(id)}" rows="${escapeHtml(rows)}"${
        placeholder ? ` placeholder="${escapeHtml(placeholder)}"` : ""
      }${required}>${escapeHtml(value)}</textarea>`;
    }
    case "select": {
      const name = prop(node, "name", "");
      const id = prop(node, "id", name);
      const value = prop(node, "value", "");
      const options = asArray(node.options);
      return `<select class="nxd-select" name="${escapeHtml(name)}" id="${escapeHtml(id)}">${options
        .map((opt) => {
          if (!isRecord(opt)) {
            const s = asString(opt);
            return `<option value="${escapeHtml(s)}"${s === value ? " selected" : ""}>${escapeHtml(s)}</option>`;
          }
          const v = asString(opt.value ?? opt.Value, "");
          const label = asString(opt.label ?? opt.Label ?? v, v);
          return `<option value="${escapeHtml(v)}"${v === value ? " selected" : ""}>${escapeHtml(label)}</option>`;
        })
        .join("\n")}</select>`;
    }
    case "checkbox": {
      const name = prop(node, "name", "");
      const id = prop(node, "id", name);
      const value = prop(node, "value", "on");
      const checked = boolAttr(node, "checked") ? " checked" : "";
      const label = prop(node, "label", "");
      const input = `<input class="nxd-checkbox" type="checkbox" name="${escapeHtml(name)}" id="${escapeHtml(id)}" value="${escapeHtml(value)}"${checked}>`;
      if (!label) {
        return input;
      }
      return `<label class="nxd-check">${input}<span>${escapeHtml(label)}</span></label>`;
    }
    case "fieldset": {
      const legend = prop(node, "legend", prop(node, "label", ""));
      return `<fieldset class="nxd-fieldset">${
        legend ? `<legend>${escapeHtml(legend)}</legend>` : ""
      }${renderChildren(node, theme)}</fieldset>`;
    }
    case "image": {
      const src = prop(node, "src", "");
      const alt = prop(node, "alt", "");
      const width = prop(node, "width", "");
      const height = prop(node, "height", "");
      const cls = classNames("nxd-img", prop(node, "class"));
      return `<img class="${cls}" src="${escapeHtml(src)}" alt="${escapeHtml(alt)}"${
        width ? ` width="${escapeHtml(width)}"` : ""
      }${height ? ` height="${escapeHtml(height)}"` : ""} decoding="async">`;
    }
    case "figure": {
      const caption = prop(node, "caption", prop(node, "figcaption", ""));
      return `<figure class="nxd-figure">${renderChildren(node, theme)}${
        caption ? `<figcaption>${escapeHtml(caption)}</figcaption>` : ""
      }</figure>`;
    }
    case "footer": {
      return `<footer class="nxd-footer"><div class="nxd-footer-inner">${renderChildren(node, theme)}</div></footer>`;
    }
    case "sidebar": {
      const id = prop(node, "id", "sidebar");
      const label = prop(node, "label", "Sidebar");
      const hiddenMobile = node.hidden_mobile !== false;
      return `<aside class="nxd-sidebar" id="${escapeHtml(id)}" aria-label="${escapeHtml(label)}"${
        hiddenMobile ? ' data-nxd-sidebar="1"' : ""
      }>${renderChildren(node, theme)}</aside>`;
    }
    case "card": {
      const href = prop(node, "href", "");
      const inner = renderChildren(node, theme);
      if (href) {
        return `<a class="nxd-card nxd-card-link" href="${escapeHtml(href)}">${inner}</a>`;
      }
      return `<div class="nxd-card">${inner}</div>`;
    }
    case "alert":
    case "banner": {
      const variant = prop(node, "variant", "info");
      const content = prop(node, "content", prop(node, "text", ""));
      const role = variant === "error" || variant === "danger" ? "alert" : "status";
      const body = childrenOf(node).length
        ? renderChildren(node, theme)
        : escapeHtml(content);
      return `<div class="nxd-alert nxd-alert-${escapeHtml(variant)}" role="${role}">${body}</div>`;
    }
    case "table": {
      const caption = prop(node, "caption", "");
      const head = asArray(node.head ?? node.headers);
      const rows = asArray(node.rows).length ? asArray(node.rows) : childrenOf(node);
      let thead = "";
      if (head.length) {
        thead = `<thead><tr>${head
          .map((c) => `<th>${escapeHtml(asString(c))}</th>`)
          .join("")}</tr></thead>`;
      }
      const tbody = `<tbody>${rows
        .map((row) => {
          if (isRecord(row) && (row._design === "tr" || row.cells)) {
            return renderNode(row, theme);
          }
          if (Array.isArray(row)) {
            return `<tr>${row.map((c) => `<td>${escapeHtml(asString(c))}</td>`).join("")}</tr>`;
          }
          return renderNode(row, theme);
        })
        .join("\n")}</tbody>`;
      return `<div class="nxd-table-wrap"><table class="nxd-table">${
        caption ? `<caption>${escapeHtml(caption)}</caption>` : ""
      }${thead}${tbody}</table></div>`;
    }
    case "tr": {
      const cells = asArray(node.cells);
      const head = node.head === true;
      const tag = head ? "th" : "td";
      return `<tr>${cells
        .map((c) => {
          if (isRecord(c) && (c._design === "td" || c._design === "th")) {
            return renderNode(c, theme);
          }
          return `<${tag}>${
            isRecord(c) && c._design ? renderNode(c, theme) : escapeHtml(asString(c))
          }</${tag}>`;
        })
        .join("")}</tr>`;
    }
    case "td":
    case "th": {
      const content = prop(node, "content", prop(node, "text", ""));
      const tag = kind;
      if (childrenOf(node).length) {
        return `<${tag}>${renderChildren(node, theme)}</${tag}>`;
      }
      return `<${tag}>${escapeHtml(content)}</${tag}>`;
    }
    case "raw": {
      return asString(node.html ?? node.content, "");
    }
    default: {
      if (childrenOf(node).length) {
        return `<div class="nxd-box nxd-${escapeHtml(kind)}">${renderChildren(node, theme)}</div>`;
      }
      return "";
    }
  }
}

/** Progressive enhancement kit — search shortcut, copy, mobile/docs drawers. */
export function nxdKitScript(): string {
  return `(function(){
  var search=document.querySelector("[data-search-input]");
  if(search){
    window.addEventListener("keydown",function(event){
      var meta=event.metaKey||event.ctrlKey;
      if((meta&&event.key.toLowerCase()==="k")||(event.key==="/"&&!meta)){
        var tag=document.activeElement&&document.activeElement.tagName;
        if(event.key==="/"&&(tag==="INPUT"||tag==="TEXTAREA"||(document.activeElement&&document.activeElement.isContentEditable))){return;}
        if(event.key==="/"&&document.activeElement===search){return;}
        event.preventDefault();
        search.focus();
        if(search.select)search.select();
      }
    });
  }
  document.querySelectorAll(".search-kbd,[data-search-kbd]").forEach(function(kbd){
    var isMac=/Mac|iPhone|iPad|iPod/.test(navigator.platform||"");
    kbd.textContent=isMac?"⌘K":"Ctrl K";
  });
  document.querySelectorAll("[data-copy]").forEach(function(button){
    button.addEventListener("click",function(){
      var value=button.getAttribute("data-copy");
      if(!value)return;
      var original=button.textContent;
      function ok(){button.textContent="Copied";window.setTimeout(function(){button.textContent=original;},1400);}
      function fail(){button.textContent="Copy failed";window.setTimeout(function(){button.textContent=original;},1400);}
      if(navigator.clipboard&&navigator.clipboard.writeText){
        navigator.clipboard.writeText(value).then(ok).catch(fail);
      }else{fail();}
    });
  });
  var body=document.body;
  var mq=window.matchMedia("(max-width: 799px)");
  var isDocs=body.classList.contains("docs-mode")||body.classList.contains("nxd-docs");
  var mobileNav=document.getElementById("mobile-nav");
  var docsSidebar=document.getElementById("sidebar");
  var navToggle=document.getElementById("nav-toggle");
  var navClose=document.getElementById("nav-close");
  var docsClose=document.getElementById("docs-nav-close");
  var navBackdrop=document.getElementById("nav-backdrop");
  var ACCORDION_KEY="nex-docs-accordion";
  var openClass=isDocs?"sidebar-open":"nav-open";
  function setDrawerOpen(open){
    body.classList.toggle(openClass,open);
    if(navToggle)navToggle.setAttribute("aria-expanded",open?"true":"false");
    if(isDocs){
      if(docsSidebar){
        if(open)docsSidebar.removeAttribute("hidden");
        else if(mq.matches)docsSidebar.setAttribute("hidden","");
        else docsSidebar.removeAttribute("hidden");
      }
    }else if(mobileNav){
      if(open)mobileNav.removeAttribute("hidden");
      else mobileNav.setAttribute("hidden","");
    }
    if(navBackdrop){
      if(open)navBackdrop.removeAttribute("hidden");
      else navBackdrop.setAttribute("hidden","");
    }
  }
  function isDrawerOpen(){return body.classList.contains(openClass);}
  if(navToggle)navToggle.addEventListener("click",function(){setDrawerOpen(!isDrawerOpen());});
  if(navClose)navClose.addEventListener("click",function(){setDrawerOpen(false);});
  if(docsClose)docsClose.addEventListener("click",function(){setDrawerOpen(false);});
  if(navBackdrop)navBackdrop.addEventListener("click",function(){setDrawerOpen(false);});
  document.addEventListener("keydown",function(event){
    if(event.key==="Escape"&&isDrawerOpen())setDrawerOpen(false);
  });
  if(mq.addEventListener){
    mq.addEventListener("change",function(){
      setDrawerOpen(false);
      if(isDocs&&docsSidebar){
        if(mq.matches)docsSidebar.setAttribute("hidden","");
        else docsSidebar.removeAttribute("hidden");
      }
    });
  }
  if(isDocs&&docsSidebar&&mq.matches)docsSidebar.setAttribute("hidden","");
  if(docsSidebar){
    var saved={};
    try{saved=JSON.parse(localStorage.getItem(ACCORDION_KEY)||"{}")||{};}catch(e){saved={};}
    var activeSection=docsSidebar.getAttribute("data-active-section");
    docsSidebar.querySelectorAll("details.sidebar-section,details.nxd-sidebar-section").forEach(function(details){
      var id=details.getAttribute("data-section");
      if(!id)return;
      if(id===activeSection)details.open=true;
      else if(Object.prototype.hasOwnProperty.call(saved,id))details.open=!!saved[id];
      details.addEventListener("toggle",function(){
        saved[id]=details.open;
        try{localStorage.setItem(ACCORDION_KEY,JSON.stringify(saved));}catch(e){}
      });
    });
  }
})();`;
}

export function themeToCss(theme: ThemeTokens): string {
  const darkBlock = `
  --nxd-ink: ${theme.darkInk};
  --nxd-ink-soft: ${theme.darkInkSoft};
  --nxd-paper: ${theme.darkPaper};
  --nxd-mist: ${theme.darkMist};
  --nxd-surface: ${theme.darkSurface};
  --nxd-line: ${theme.darkLine};
  --nxd-accent: ${theme.darkAccent};
  --nxd-accent-deep: ${theme.darkAccentDeep};
  --nxd-accent-soft: ${theme.darkAccentSoft};
  --nxd-on-accent: ${theme.darkOnAccent};
`;

  const mode = theme.mode || "system";
  let darkCss = "";
  if (mode === "dark") {
    darkCss = `html, body.nxd-body {${darkBlock}}`;
  } else if (mode !== "light") {
    darkCss = `@media (prefers-color-scheme: dark) {
  :root {${darkBlock}}
  body.nxd-body::before {
    background:
      radial-gradient(720px 320px at 10% -10%, rgba(59, 130, 246, 0.08), transparent 58%),
      radial-gradient(560px 260px at 100% 0%, rgba(232, 238, 247, 0.03), transparent 55%);
  }
  .nxd-codeblock { background: #020617; }
}
html[data-theme="dark"], body.nxd-body[data-theme="dark"] {${darkBlock}}`;
  }

  return `:root {
  --nxd-ink: ${theme.ink};
  --nxd-ink-soft: ${theme.inkSoft};
  --nxd-paper: ${theme.paper};
  --nxd-mist: ${theme.mist};
  --nxd-surface: ${theme.surface};
  --nxd-line: ${theme.line};
  --nxd-accent: ${theme.accent};
  --nxd-accent-deep: ${theme.accentDeep};
  --nxd-accent-soft: ${theme.accentSoft};
  --nxd-on-accent: ${theme.onAccent};
  --nxd-font-display: ${theme.fontDisplay};
  --nxd-font-body: ${theme.fontBody};
  --nxd-font-mono: ${theme.fontMono};
  --nxd-radius: ${theme.radius};
  --nxd-max: ${theme.max};
  --nxd-ease: ${theme.ease};
  --nxd-bp-sm: ${theme.bpSm};
  --nxd-bp-md: ${theme.bpMd};
  --nxd-bp-lg: ${theme.bpLg};
}

*, *::before, *::after { box-sizing: border-box; }

html { scroll-behavior: smooth; }

body.nxd-body {
  margin: 0;
  min-height: 100vh;
  color: var(--nxd-ink);
  font-family: var(--nxd-font-body);
  font-size: 1rem;
  line-height: 1.55;
  background: var(--nxd-paper);
  overflow-x: hidden;
  display: flex;
  flex-direction: column;
}

body.nxd-body::before {
  content: "";
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: -1;
  background:
    radial-gradient(720px 320px at 10% -10%, rgba(29, 78, 216, 0.06), transparent 58%),
    radial-gradient(560px 260px at 100% 0%, rgba(15, 23, 42, 0.04), transparent 55%);
}

main.nxd-main { flex: 1 0 auto; }

a { color: var(--nxd-accent-deep); text-underline-offset: 0.18em; }
a:hover { color: var(--nxd-accent); }

.nxd-shell {
  width: min(100% - 2rem, var(--nxd-max));
  margin-inline: auto;
}

.nxd-topbar {
  position: sticky;
  top: 0;
  z-index: 20;
  backdrop-filter: blur(10px);
  background: color-mix(in srgb, var(--nxd-paper) 86%, transparent);
  border-bottom: 1px solid var(--nxd-line);
}

.nxd-topbar-inner {
  width: min(100% - 2rem, var(--nxd-max));
  margin-inline: auto;
  min-height: 3.5rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.nxd-topbar-end {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.nxd-brand {
  display: inline-flex;
  align-items: center;
  gap: 0.7rem;
  text-decoration: none;
  color: inherit;
}

.nxd-brand-mark { border-radius: 0.35rem; }

.nxd-brand-text {
  display: flex;
  flex-direction: column;
  line-height: 1.1;
}

.nxd-brand-name {
  font-family: var(--nxd-font-display);
  font-weight: 800;
  font-size: 1.15rem;
  letter-spacing: -0.02em;
}

.nxd-brand-tag {
  font-size: 0.72rem;
  color: var(--nxd-ink-soft);
  font-weight: 550;
}

.nxd-nav {
  display: flex;
  flex-wrap: wrap;
  gap: 0.85rem;
  align-items: center;
}

.nxd-nav-link {
  text-decoration: none;
  color: var(--nxd-ink-soft);
  font-weight: 550;
  font-size: 0.95rem;
}

.nxd-nav-link[aria-current="page"],
.nxd-nav-link:hover { color: var(--nxd-ink); }

.nxd-nav-toggle {
  display: none;
  flex-direction: column;
  justify-content: center;
  gap: 4px;
  width: 2.4rem;
  height: 2.4rem;
  padding: 0.45rem;
  border: 1px solid var(--nxd-line);
  border-radius: var(--nxd-radius);
  background: var(--nxd-surface);
  cursor: pointer;
}

.nxd-nav-toggle span {
  display: block;
  height: 2px;
  background: var(--nxd-ink);
  border-radius: 1px;
}

.nxd-mobile-nav {
  display: none;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1rem 1.25rem 1.25rem;
  border-top: 1px solid var(--nxd-line);
  background: var(--nxd-surface);
}

.nxd-mobile-nav .nxd-nav {
  flex-direction: column;
  align-items: flex-start;
}

.nxd-nav-backdrop {
  display: none;
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.35);
  z-index: 15;
}

body.nav-open .nxd-nav-backdrop { display: block; }
body.nav-open .nxd-mobile-nav { display: flex; position: relative; z-index: 16; }

.nxd-nav-close {
  align-self: flex-start;
  border: 1px solid var(--nxd-line);
  background: var(--nxd-paper);
  border-radius: var(--nxd-radius);
  padding: 0.4rem 0.75rem;
  cursor: pointer;
  font: inherit;
}

.nxd-hero {
  width: min(100% - 2rem, var(--nxd-max));
  margin: 0 auto;
  padding: clamp(2.5rem, 8vw, 5rem) 0 clamp(2rem, 5vw, 3.5rem);
  display: grid;
  gap: 1.25rem;
  animation: nxd-rise 700ms var(--nxd-ease) both;
}

.nxd-stack { display: flex; flex-direction: column; }
.nxd-row { display: flex; flex-direction: row; }
.nxd-grid { display: grid; }

.nxd-text-kicker {
  margin: 0;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  font-size: 0.78rem;
  font-weight: 700;
  color: var(--nxd-accent-deep);
}

.nxd-text-headline {
  margin: 0;
  max-width: 18ch;
  font-family: var(--nxd-font-display);
  font-weight: 800;
  font-size: clamp(2.4rem, 6vw, 4.2rem);
  line-height: 1.05;
  letter-spacing: -0.03em;
  animation: nxd-rise 800ms var(--nxd-ease) 60ms both;
}

.nxd-text-headline em {
  font-style: normal;
  color: var(--nxd-accent);
}

.nxd-text-lead {
  margin: 0;
  max-width: 38rem;
  font-size: 1.15rem;
  color: var(--nxd-ink-soft);
  animation: nxd-rise 850ms var(--nxd-ease) 120ms both;
}

.nxd-text-body { margin: 0; color: var(--nxd-ink); }
.nxd-text-code {
  font-family: var(--nxd-font-mono);
  font-size: 0.92em;
  background: var(--nxd-accent-soft);
  padding: 0.1em 0.35em;
  border-radius: 0.25rem;
}

.nxd-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.6rem;
  padding: 0.55rem 1.1rem;
  border-radius: var(--nxd-radius);
  text-decoration: none;
  font-weight: 650;
  font: inherit;
  font-weight: 650;
  border: 0;
  cursor: pointer;
  transition: transform 160ms var(--nxd-ease), background 160ms var(--nxd-ease);
}

.nxd-btn:hover { transform: translateY(-1px); }

.nxd-btn-primary {
  background: var(--nxd-accent);
  color: var(--nxd-on-accent);
}

.nxd-btn-primary:hover { background: var(--nxd-accent-deep); color: var(--nxd-on-accent); }

.nxd-btn-secondary {
  background: var(--nxd-surface);
  color: var(--nxd-ink);
  border: 1px solid var(--nxd-line);
}

.nxd-btn-ghost {
  background: transparent;
  color: var(--nxd-ink-soft);
  border: 1px solid transparent;
}

.nxd-link { font-weight: 600; }

.nxd-rule {
  border: 0;
  border-top: 1px solid var(--nxd-line);
  margin: 0;
}

.nxd-codeblock {
  margin: 0;
  padding: 1rem 1.1rem;
  overflow: auto;
  border-radius: var(--nxd-radius);
  background: #0b1220;
  color: #e8eef7;
  border: 1px solid rgba(232, 238, 247, 0.08);
  font-family: var(--nxd-font-mono);
  font-size: 0.88rem;
  line-height: 1.5;
}

.nxd-list {
  margin: 0;
  padding-left: 1.2rem;
  color: var(--nxd-ink-soft);
}

.nxd-list li { margin: 0.35rem 0; }

.nxd-section {
  width: min(100% - 2rem, var(--nxd-max));
  margin-inline: auto;
  padding: 2rem 0 3rem;
}

.nxd-form, .nxd-fieldset {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
  max-width: 28rem;
}

.nxd-fieldset {
  border: 1px solid var(--nxd-line);
  border-radius: var(--nxd-radius);
  padding: 1rem;
  margin: 0;
}

.nxd-field, .nxd-label {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  font-weight: 550;
}

.nxd-field-label { color: var(--nxd-ink); font-size: 0.92rem; }

.nxd-input, .nxd-textarea, .nxd-select {
  font: inherit;
  color: var(--nxd-ink);
  background: var(--nxd-surface);
  border: 1px solid var(--nxd-line);
  border-radius: var(--nxd-radius);
  padding: 0.55rem 0.75rem;
  min-height: 2.5rem;
}

.nxd-textarea { min-height: 6rem; resize: vertical; }

.nxd-input:focus, .nxd-textarea:focus, .nxd-select:focus {
  outline: 2px solid var(--nxd-accent-soft);
  border-color: var(--nxd-accent);
}

.nxd-check {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 500;
}

.nxd-checkbox { width: 1.05rem; height: 1.05rem; accent-color: var(--nxd-accent); }

.nxd-img { max-width: 100%; height: auto; display: block; border-radius: var(--nxd-radius); }

.nxd-figure { margin: 0; }
.nxd-figure figcaption {
  margin-top: 0.5rem;
  font-size: 0.9rem;
  color: var(--nxd-ink-soft);
}

.nxd-footer {
  margin-top: auto;
  border-top: 1px solid var(--nxd-line);
  background: color-mix(in srgb, var(--nxd-surface) 80%, var(--nxd-paper));
}

.nxd-footer-inner {
  width: min(100% - 2rem, var(--nxd-max));
  margin-inline: auto;
  padding: 1.75rem 0 2.25rem;
  display: grid;
  gap: 1rem;
  color: var(--nxd-ink-soft);
  font-size: 0.92rem;
}

.nxd-layout {
  width: min(100% - 2rem, var(--nxd-max));
  margin: 0 auto;
  padding: 1.5rem 0 3rem;
}

.nxd-layout-aside {
  display: grid;
  grid-template-columns: 15rem minmax(0, 1fr);
  gap: 1.75rem;
  align-items: start;
}

.nxd-sidebar {
  position: sticky;
  top: 4.25rem;
  padding: 0.75rem;
  border: 1px solid var(--nxd-line);
  border-radius: var(--nxd-radius);
  background: var(--nxd-surface);
}

.nxd-card {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  padding: 1rem 1.1rem;
  border: 1px solid var(--nxd-line);
  border-radius: var(--nxd-radius);
  background: var(--nxd-surface);
  text-decoration: none;
  color: inherit;
  transition: border-color 160ms var(--nxd-ease), transform 160ms var(--nxd-ease);
}

.nxd-card-link:hover {
  border-color: var(--nxd-accent);
  transform: translateY(-1px);
}

.nxd-alert {
  padding: 0.75rem 1rem;
  border-radius: var(--nxd-radius);
  border: 1px solid var(--nxd-line);
  background: var(--nxd-accent-soft);
  color: var(--nxd-ink);
}

.nxd-alert-error, .nxd-alert-danger {
  background: rgba(185, 28, 28, 0.1);
  border-color: rgba(185, 28, 28, 0.35);
  color: #991b1b;
}

.nxd-alert-success {
  background: rgba(21, 128, 61, 0.1);
  border-color: rgba(21, 128, 61, 0.3);
  color: #166534;
}

.nxd-alert-warning {
  background: rgba(161, 98, 7, 0.1);
  border-color: rgba(161, 98, 7, 0.3);
  color: #92400e;
}

.nxd-table-wrap { overflow-x: auto; }

.nxd-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.95rem;
}

.nxd-table th, .nxd-table td {
  text-align: left;
  padding: 0.65rem 0.75rem;
  border-bottom: 1px solid var(--nxd-line);
}

.nxd-table th {
  font-weight: 650;
  color: var(--nxd-ink-soft);
  font-size: 0.82rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.nxd-search-form {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  max-width: 40rem;
  align-items: stretch;
}

.nxd-search-form .nxd-input { flex: 1 1 14rem; }

.nxd-pkg-name {
  font-family: var(--nxd-font-display);
  font-weight: 750;
  font-size: 1.05rem;
}

.nxd-muted { color: var(--nxd-ink-soft); }

.nxd-visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0,0,0,0);
  white-space: nowrap;
  border: 0;
}

@keyframes nxd-rise {
  from { opacity: 0; transform: translateY(12px); }
  to { opacity: 1; transform: translateY(0); }
}

@media (max-width: 799px) {
  .nxd-topbar-inner .nxd-nav { display: none; }
  .nxd-nav-toggle { display: inline-flex; }
  .nxd-layout-aside { grid-template-columns: 1fr; }
  .nxd-sidebar[data-nxd-sidebar="1"] {
    position: fixed;
    inset: 0 auto 0 0;
    width: min(18rem, 88vw);
    z-index: 30;
    border-radius: 0;
    transform: translateX(-105%);
    transition: transform 200ms var(--nxd-ease);
  }
  body.sidebar-open .nxd-sidebar[data-nxd-sidebar="1"] {
    transform: translateX(0);
  }
  body.sidebar-open .nxd-nav-backdrop { display: block; z-index: 25; }
}

@media (max-width: 640px) {
  .nxd-topbar-inner { flex-wrap: wrap; padding-block: 0.6rem; }
  .nxd-text-headline { max-width: none; }
}

${darkCss}
`;
}

export interface DesignDocumentOptions {
  title?: string;
  description?: string;
  theme?: ThemeTokens;
  bodyHtml?: string;
  headExtra?: string;
  kit?: boolean;
  bodyClass?: string;
}

export function renderDesignTree(tree: DesignData, themeInput?: DesignData): string {
  let theme = themeInput ? themeFromData(themeInput) : NEXUS_DEFAULT_THEME;
  if (isRecord(tree) && (tree._design === "page" || tree.theme)) {
    if (tree.theme) {
      theme = themeFromData(tree.theme);
    }
  }
  return renderNode(tree, theme);
}

export function renderDesignDocument(tree: DesignData): string {
  let theme = NEXUS_DEFAULT_THEME;
  let title = "Nexus";
  let description =
    "Nexus design language — declarative layout, theme, and components in .nex.";
  let bodyTree: DesignData = tree;
  let topbar = "";
  let footer = "";
  let kit = true;
  let bodyClass = "nxd-body";
  let modeAttr = "";

  if (isRecord(tree) && tree._design === "page") {
    if (tree.theme) {
      theme = themeFromData(tree.theme);
    }
    title = asString(tree.title, title);
    description = asString(tree.description, description);
    if (tree.topbar) {
      topbar = renderNode(tree.topbar, theme);
    }
    if (tree.footer) {
      footer = renderNode(tree.footer, theme);
    }
    bodyTree = tree.body ?? tree.content ?? tree.children ?? [];
    if (tree.kit === false) {
      kit = false;
    }
    if (tree.docs === true || tree.docs_mode === true) {
      bodyClass += " docs-mode nxd-docs";
    }
    if (asString(tree.body_class)) {
      bodyClass += " " + asString(tree.body_class);
    }
  } else if (isRecord(tree) && tree.theme) {
    theme = themeFromData(tree.theme);
  }

  if (theme.mode === "dark" || theme.mode === "light") {
    modeAttr = ` data-theme="${theme.mode}"`;
  }

  // If body is a layout node that already includes topbar/footer, don't double-wrap chrome.
  if (isRecord(bodyTree) && bodyTree._design === "layout") {
    if (bodyTree.topbar) {
      topbar = "";
    }
    if (bodyTree.footer) {
      footer = "";
    }
    if (bodyTree.docs === true || bodyTree.docs_mode === true) {
      bodyClass += " docs-mode nxd-docs";
    }
  }

  const css = themeToCss(theme);
  const body = renderNode(bodyTree, theme);
  const script = kit
    ? `\n<script>\n${nxdKitScript()}\n</script>`
    : "";

  return `<!DOCTYPE html>
<html lang="en"${modeAttr}>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>${escapeHtml(title)}</title>
  <meta name="description" content="${escapeHtml(description)}">
  <meta name="color-scheme" content="light dark">
  <link rel="icon" href="/static/img/favicon.svg?v=20260811-logo" type="image/svg+xml">
  <link rel="apple-touch-icon" href="/static/img/logo.png?v=20260811-logo" sizes="180x180">
  <meta property="og:image" content="/static/img/logo.png?v=20260811-logo">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Bricolage+Grotesque:opsz,wght@12..96,500;12..96,700;12..96,800&family=Figtree:wght@400;550;650;700&family=IBM+Plex+Mono:wght@400;500&display=swap" rel="stylesheet">
  <style>
${css}
  </style>
</head>
<body class="${escapeHtml(bodyClass.trim())}"${modeAttr}>
${topbar}
<main class="nxd-main">
${body}
</main>
${footer}
${script}
</body>
</html>
`;
}
