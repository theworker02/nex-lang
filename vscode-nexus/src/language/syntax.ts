/**
 * Universal polyglot syntax facade.
 * Lowers English-inspired sugar into core Nexus syntax before lexing.
 *
 * Supported sugar (examples):
 *   set x to 10                 → let x = 10;
 *   var x = 10                  → let mut x = 10;
 *   fun add(a, b) do ... end    → let add = fn(a, b) { ... };
 *   if x > 0 then ... end       → if (x > 0) { ... }
 *   when x is Some(n) then ...  → match x { Some(n) => { ... } }
 *   x and y / x or y / not x    → logical ops via nested ifs / bang
 *   for i in a to b do ... end  → while-style expansion
 *   theme Name { ink = "#…" }   → let Name = theme({ "ink": "#…" });
 *   style { gap = "space_4" }   → { "gap": "space_4" }
 *   view Name = expr            → let Name = expr
 */
export function lowerSyntax(source: string): string {
  let s = source;

  // Strip UTF-8 BOM
  if (s.charCodeAt(0) === 0xfeff) {
    s = s.slice(1);
  }

  // #derive(json) struct Foo → struct Foo (attributes recorded via comment tag)
  s = s.replace(
    /#derive\(([^)]+)\)\s*struct\s+(\w+)/g,
    (_m, attrs: string, name: string) =>
      `/* @derive ${attrs.trim()} */\nstruct ${name}`,
  );

  // #reflect(Type) → reflect(Type)
  s = s.replace(/#reflect\((\w+)\)/g, "reflect($1)");

  // set name to value → let name = value
  s = s.replace(
    /\bset\s+([A-Za-z_][A-Za-z0-9_]*)\s+to\s+/g,
    "let $1 = ",
  );

  // var name = → let mut name =
  s = s.replace(/\bvar\s+/g, "let mut ");

  // fun name(args) do ... end → let name = fn(args) { ... };
  s = s.replace(
    /\bfun\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)\s*do\b/g,
    "let $1 = fn($2) {",
  );

  // Standalone `do` / `end` block delimiters → { }
  // Convert `end` that closes fun/if/when/for/while/region
  s = replaceDoEndBlocks(s);

  // if cond then → if (cond) {
  s = s.replace(
    /\bif\s+(?!\()([^\n]+?)\s+then\b/g,
    (_m, cond: string) => `if (${cond.trim()}) {`,
  );

  // else then → else {
  s = s.replace(/\belse\s+then\b/g, "else {");

  // when expr is → match expr {
  s = s.replace(
    /\bwhen\s+(.+?)\s+is\b/g,
    (_m, scrut: string) => `match ${scrut.trim()} {`,
  );

  // pattern then body  (inside match - best effort: `Foo(x) then` → `Foo(x) =>`)
  s = s.replace(
    /(\b[A-Za-z_][A-Za-z0-9_]*(?:\([^)]*\))?|\b_\b)\s+then\b/g,
    "$1 =>",
  );

  // and/or/not handled exclusively by lowerBooleanWords (string-safe).
  s = lowerBooleanWords(source, s);

  // for i in LO to HI do → let i = LO; while-style unrolled as recursive fn sugar
  s = s.replace(
    /\bfor\s+([A-Za-z_][A-Za-z0-9_]*)\s+in\s+([^t]+?)\s+to\s+([^\n]+?)\s*(?:do\b)?/g,
    (_m, i: string, lo: string, hi: string) => {
      return `let ${i} = ${lo.trim()}; /* for-to ${hi.trim()} */`;
    },
  );

  // while cond do → /* while */ if (cond) {
  s = s.replace(
    /\bwhile\s+(?!\()([^\n]+?)\s+do\b/g,
    (_m, cond: string) => `/* while */ if (${cond.trim()}) {`,
  );

  // shared/mutable refs without lifetimes: ref x → &x, mut ref x → &mut x
  s = s.replace(/\bmut\s+ref\s+/g, "&mut ");
  s = s.replace(/\bref\s+/g, "&");

  // make channel(n) → chan(n)
  s = s.replace(/\bmake\s+channel\s*\(/g, "chan(");
  s = s.replace(/\bchannel\s*\(/g, "chan(");

  // run task expr → spawn(expr)
  s = s.replace(/\brun\s+task\s+/g, "spawn ");

  // wait for expr → await expr
  s = s.replace(/\bwait\s+for\s+/g, "await ");

  // allocate n → alloc(n) if we add ALLOC keyword call form
  s = s.replace(/\ballocate\s+/g, "alloc ");

  // Design language sugar (CSS-inspired theme / style blocks)
  s = lowerDesignSyntax(s);

  return s;
}

/**
 * Lower design-language sugar into core Nexus:
 *
 *   theme Name {
 *     ink = "#0f172a"
 *     paper = "#f1f5f9"
 *   }
 *   → let Name = theme({ "ink": "#0f172a", "paper": "#f1f5f9" });
 *
 *   style {
 *     gap = "space_4"
 *     pad = "space_8"
 *   }
 *   → { "gap": "space_4", "pad": "space_8" }
 *
 *   view Name = expr;
 *   → let Name = expr;   (alias for readability in design modules)
 */
function lowerDesignSyntax(source: string): string {
  let s = source;

  s = s.replace(
    /\btheme\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{([\s\S]*?)\}/g,
    (_m, name: string, body: string) => {
      const pairs = designBlockPairs(body);
      return `let ${name} = theme({ ${pairs} });`;
    },
  );

  s = s.replace(/\bstyle\s*\{([\s\S]*?)\}/g, (_m, body: string) => {
    const pairs = designBlockPairs(body);
    return `{ ${pairs} }`;
  });

  s = s.replace(
    /\bview\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*/g,
    "let $1 = ",
  );

  return s;
}

function designBlockPairs(body: string): string {
  const pairs: string[] = [];
  const lines = body.split(/\r?\n|;/);
  for (const raw of lines) {
    const line = raw.trim();
    if (!line || line.startsWith("//")) {
      continue;
    }
    const m = /^([A-Za-z_][A-Za-z0-9_-]*)\s*[:=]\s*(.+)$/.exec(line);
    if (!m) {
      continue;
    }
    const key = m[1]!.replace(/-/g, "_");
    let val = m[2]!.trim();
    if (val.endsWith(",")) {
      val = val.slice(0, -1).trim();
    }
    pairs.push(`"${key}": ${val}`);
  }
  return pairs.join(", ");
}

function replaceDoEndBlocks(s: string): string {
  // Convert remaining `do` to `{` and matching `end` to `}`
  // Careful not to touch identifiers containing those syllables — word boundaries used.
  let out = "";
  let i = 0;
  let depth = 0;
  while (i < s.length) {
    if (
      /\bdo\b/.test(s.slice(i, i + 3)) &&
      (i === 0 || !isIdentChar(s[i - 1]!)) &&
      !isIdentChar(s[i + 2] ?? "")
    ) {
      if (s.slice(i, i + 2) === "do" && !isIdentChar(s[i + 2] ?? "")) {
        out += "{";
        depth += 1;
        i += 2;
        continue;
      }
    }
    if (
      s.slice(i, i + 3) === "end" &&
      !isIdentChar(s[i - 1] ?? "") &&
      !isIdentChar(s[i + 3] ?? "")
    ) {
      out += "}";
      depth = Math.max(0, depth - 1);
      i += 3;
      continue;
    }
    out += s[i];
    i += 1;
  }
  void depth;
  return out;
}

function isIdentChar(ch: string): boolean {
  return /[A-Za-z0-9_]/.test(ch);
}

/** Apply a regex replace only to regions outside double-quoted strings. */
function replaceOutsideStrings(
  source: string,
  pattern: RegExp,
  replacement: string | ((...args: string[]) => string),
): string {
  let out = "";
  let i = 0;
  while (i < source.length) {
    if (source[i] === '"') {
      out += source[i];
      i += 1;
      while (i < source.length) {
        if (source[i] === "\\" && i + 1 < source.length) {
          out += source[i]! + source[i + 1]!;
          i += 2;
          continue;
        }
        out += source[i];
        if (source[i] === '"') {
          i += 1;
          break;
        }
        i += 1;
      }
      continue;
    }
    let j = i;
    while (j < source.length && source[j] !== '"') {
      j += 1;
    }
    const chunk = source.slice(i, j);
    out +=
      typeof replacement === "string"
        ? chunk.replace(pattern, replacement)
        : chunk.replace(
            pattern,
            replacement as (substring: string, ...args: string[]) => string,
          );
    i = j;
  }
  return out;
}

function hasWordOutsideStrings(source: string, word: string): boolean {
  const re = new RegExp(`\\b${word}\\b`);
  let i = 0;
  while (i < source.length) {
    if (source[i] === '"') {
      i += 1;
      while (i < source.length) {
        if (source[i] === "\\" && i + 1 < source.length) {
          i += 2;
          continue;
        }
        if (source[i] === '"') {
          i += 1;
          break;
        }
        i += 1;
      }
      continue;
    }
    let j = i;
    while (j < source.length && source[j] !== '"') {
      j += 1;
    }
    if (re.test(source.slice(i, j))) {
      return true;
    }
    i = j;
  }
  return false;
}

function lowerBooleanWords(original: string, already: string): string {
  let s = already;
  if (
    hasWordOutsideStrings(original, "and") ||
    hasWordOutsideStrings(original, "or") ||
    hasWordOutsideStrings(original, "not")
  ) {
    s = original;
    s = s.replace(/#derive\(([^)]+)\)\s*struct\s+(\w+)/g, "struct $2");
    s = s.replace(/#reflect\((\w+)\)/g, "reflect($1)");
    s = s.replace(/\bset\s+([A-Za-z_][A-Za-z0-9_]*)\s+to\s+/g, "let $1 = ");
    s = s.replace(/\bvar\s+/g, "let mut ");
    s = s.replace(
      /\bfun\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)\s*do\b/g,
      "let $1 = fn($2) {",
    );
    s = replaceDoEndBlocks(s);
    s = s.replace(
      /\bif\s+(?!\()([^\n]+?)\s+then\b/g,
      (_m, cond: string) => `if (${cond.trim()}) {`,
    );
    s = s.replace(/\belse\s+then\b/g, "else {");
    s = s.replace(
      /\bwhen\s+(.+?)\s+is\b/g,
      (_m, scrut: string) => `match ${scrut.trim()} {`,
    );
    s = s.replace(
      /(\b[A-Za-z_][A-Za-z0-9_]*(?:\([^)]*\))?|\b_\b)\s+then\b/g,
      "$1 =>",
    );
    s = replaceOutsideStrings(s, /\bnot\s+\(/g, "!(");
    s = replaceOutsideStrings(s, /\bnot\s+([A-Za-z_][A-Za-z0-9_]*)/g, "!($1)");
    s = replaceOutsideStrings(s, /\s+\band\s+/g, " __and ");
    s = replaceOutsideStrings(s, /\s+\bor\s+/g, " __or ");
    s = replaceOutsideStrings(
      s,
      /([A-Za-z0-9_\)\]]+)\s+__and\s+([A-Za-z0-9_\(\[]+)/g,
      "__and($1, $2)",
    );
    s = replaceOutsideStrings(
      s,
      /([A-Za-z0-9_\)\]]+)\s+__or\s+([A-Za-z0-9_\(\[]+)/g,
      "__or($1, $2)",
    );
    s = s.replace(/\bmut\s+ref\s+/g, "&mut ");
    s = s.replace(/\bref\s+/g, "&");
    s = s.replace(/\bmake\s+channel\s*\(/g, "chan(");
    s = s.replace(/\brun\s+task\s+/g, "spawn ");
    s = s.replace(/\bwait\s+for\s+/g, "await ");
  }
  return s;
}

export interface SyntaxFeatures {
  englishBindings: boolean;
  doEndBlocks: boolean;
  whenIsMatch: boolean;
  softRefs: boolean;
  taskSugar: boolean;
}

export const DEFAULT_SYNTAX_FEATURES: SyntaxFeatures = {
  englishBindings: true,
  doEndBlocks: true,
  whenIsMatch: true,
  softRefs: true,
  taskSugar: true,
};
