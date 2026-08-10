/**
 * Smoke test for the Go html/template subset (out/host/templates.js).
 *
 * Regression guard: define bodies must be extracted depth-aware. A non-greedy
 * {{define}}...{{end}} regex closes "base" at the first nested {{end}} (the one
 * inside {{if .Title}} in <title>), truncating every page to ~150 bytes.
 */

const { loadGoTemplates, normalizeTemplateData, SafeHtml } = require("../out/host/templates");

function assert(cond, msg) {
  if (!cond) {
    throw new Error(msg);
  }
}

// Mirrors the real registry base.html shape: the very first action inside the
// define is a nested {{if}}...{{end}} inside <title>.
const BASE = `{{define "base"}}<!DOCTYPE html>
<html>
<head><title>{{if .Title}}{{.Title}} · Nexus{{else}}Nexus{{end}}</title></head>
<body>
{{if .IsSearch}}<div id="search">{{.Query}}</div>{{end}}
{{range .Packages}}<li>{{.Name}}</li>{{end}}
{{with .User}}<span>{{.Name}}</span>{{end}}
<main>{{template "content" .}}</main>
<footer>END-OF-BASE</footer>
</body>
</html>
{{end}}`;

const PARTIAL = `{{define "sidebar"}}<aside>{{if .Tags}}{{range .Tags}}<b>{{.}}</b>{{end}}{{end}}SIDEBAR-END</aside>{{end}}`;

const PAGE = `{{define "content"}}<h1>{{.Heading}}</h1>{{if .Body}}<p>{{.Body}}</p>{{end}}{{template "sidebar" .}}CONTENT-END{{end}}`;

function main() {
  const engine = loadGoTemplates({
    base: BASE,
    partials: { sidebar: PARTIAL },
    pages: { home: PAGE },
  });

  const html = engine.render(
    "home",
    normalizeTemplateData({
      Title: "Packages",
      IsSearch: true,
      Query: "httpkit",
      Packages: [{ Name: "httpkit" }, { Name: "jsonx" }],
      User: { Name: "matth" },
      Heading: "Hello",
      Body: "World",
      Tags: ["a", "b"],
    }),
  );

  // The truncation bug cut the body off inside <title>, ~150 bytes in.
  assert(
    html.length > 250,
    `base body truncated: got ${html.length} bytes:\n${html}`,
  );
  assert(
    html.includes("END-OF-BASE"),
    "base define closed at the nested {{end}} inside <title> — extractDefines is not depth-aware",
  );
  assert(html.includes("</html>"), "base body missing closing </html>");

  // Nested if/else inside the define resolved, not swallowed.
  assert(html.includes("Packages · Nexus"), "nested {{if}}/{{else}} in <title> lost");
  assert(!html.includes("{{"), `unrendered action left in output:\n${html}`);

  // Nested range and with survived extraction.
  assert(html.includes("<li>httpkit</li>") && html.includes("<li>jsonx</li>"), "nested {{range}} lost");
  assert(html.includes("<span>matth</span>"), "nested {{with}} lost");
  assert(html.includes('<div id="search">httpkit</div>'), "nested {{if}} block lost");

  // Multiple defines across base + partial + page all extracted in full.
  assert(html.includes("CONTENT-END"), "page define truncated");
  assert(html.includes("SIDEBAR-END"), "partial define truncated (doubly-nested if/range)");
  assert(html.includes("<b>a</b><b>b</b>"), "doubly-nested range lost");

  // {{else}} branch selection.
  const noTitle = engine.render("home", normalizeTemplateData({ Heading: "H" }));
  assert(
    noTitle.includes("<title>Nexus</title>"),
    `{{else}} branch not taken: ${noTitle.slice(0, 200)}`,
  );
  assert(noTitle.includes("END-OF-BASE"), "base truncated on else path");

  // SafeHtml passthrough stays unescaped.
  const safe = engine.render(
    "home",
    normalizeTemplateData({ Heading: "H", Body: new SafeHtml("<i>raw</i>") }),
  );
  assert(safe.includes("<p><i>raw</i></p>"), "SafeHtml was escaped");

  console.log("smoke-templates: ok");
}

main();
