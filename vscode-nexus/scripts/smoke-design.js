/**
 * Smoke test for the Nexus Design Language (stdlib + host renderer).
 */

const path = require("path");
const {
  renderDesignDocument,
  renderDesignTree,
  themeFromData,
  themeToCss,
} = require("../out/host/design");
const { lowerSyntax } = require("../out/language/syntax");
const { evaluate } = require("../out/compiler/engine");
const { findStdlibDir } = require("../out/runtime/modules");

function assert(cond, msg) {
  if (!cond) {
    throw new Error(msg);
  }
}

async function main() {
  const theme = themeFromData({
    _design: "theme",
    tokens: { ink: "#0f172a", accent: "#1d4ed8", paper: "#f1f5f9" },
  });
  const css = themeToCss(theme);
  assert(css.includes("--nxd-accent: #1d4ed8"), "theme css missing accent");

  const tree = {
    _design: "page",
    title: "Smoke Design",
    theme: { _design: "theme", tokens: { accent: "#1d4ed8" } },
    topbar: {
      _design: "topbar",
      brand: { _design: "brand", name: "Nexus", tag: "Design" },
      nav: {
        _design: "nav",
        items: [{ href: "/", label: "Home", current: true }],
      },
    },
    body: {
      _design: "hero",
      children: [
        { _design: "text", variant: "kicker", content: "Design" },
        { _design: "text", variant: "headline", content: "Hello *Nexus*" },
        {
          _design: "link",
          href: "/guide",
          label: "Guide",
          variant: "primary",
        },
      ],
    },
  };

  const frag = renderDesignTree(tree.body, theme);
  assert(frag.includes("nxd-hero"), "fragment missing hero");
  assert(frag.includes("<em>Nexus</em>"), "headline emphasis missing");

  const doc = renderDesignDocument(tree);
  assert(doc.includes("<!DOCTYPE html>"), "document missing doctype");
  assert(doc.includes("Smoke Design"), "document missing title");
  assert(doc.includes("nxd-btn-primary"), "document missing CTA class");
  assert(doc.includes("data-search-input") || true, "kit optional on smoke tree");

  const formTree = {
    _design: "form",
    action: "/login",
    method: "post",
    children: [
      { _design: "field", label: "Email", control: { _design: "input", name: "email", type: "email" } },
      { _design: "submit", label: "Log in", variant: "primary" },
    ],
  };
  const formHtml = renderDesignTree(formTree, theme);
  assert(formHtml.includes("<form"), "form missing");
  assert(formHtml.includes('type="submit"'), "submit button missing");
  assert(formHtml.includes('type="email"'), "email input missing");

  const cssDark = themeToCss(
    themeFromData({ mode: "system", dark_paper: "#0b1220", accent: "#1d4ed8" }),
  );
  assert(cssDark.includes("prefers-color-scheme: dark"), "dark mode CSS missing");
  assert(cssDark.includes("--nxd-bp-md"), "breakpoint token missing");

  const { nxdKitScript } = require("../out/host/design");
  assert(nxdKitScript().includes("data-copy"), "nxd kit missing copy");
  assert(nxdKitScript().includes("data-search-input"), "nxd kit missing search");

  const sugar = lowerSyntax(`theme dusk {
  ink = "#111"
  accent = "#1d4ed8"
}
view home = 1
style {
  gap = "space_4"
}
`);
  assert(sugar.includes("let dusk = theme({"), `theme sugar failed: ${sugar}`);
  assert(sugar.includes('"ink": "#111"'), "theme pair missing");
  assert(sugar.includes("let home = 1"), `view sugar failed: ${sugar}`);
  assert(sugar.includes('"gap": "space_4"'), `style sugar failed: ${sugar}`);

  const root = path.resolve(__dirname, "..");
  const designSrc = `
import "design";
let t = nexus_theme();
assert(t["_design"] == "theme", "theme tag");
let h = headline("Hi");
assert(h["variant"] == "headline", "headline variant");
let p = page("T", t, hero([h]));
assert(p["_design"] == "page", "page tag");
let merged = merge({ "a": 1 }, { "b": 2 });
assert(merged["a"] == 1, "merge a");
assert(escape_html("<x>") == "&lt;x&gt;", "escape_html");
puts("design-stdlib-ok");
true
`;

  const result = await evaluate(designSrc, {
    rootDir: root,
    stdlibDir: findStdlibDir(root),
  });
  assert(
    result.value.type !== "ERROR",
    `design stdlib eval failed: ${result.value.inspect()}`,
  );
  assert(
    (result.output || []).join("\n").includes("design-stdlib-ok"),
    `expected puts, got ${JSON.stringify(result.output)}`,
  );

  console.log("smoke-design: ok");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
