# Syntax and expressions

## Lexical structure

- **Comments:** `//` to end of line; block comments `/* ... */` appear in sugar/macros
- **Identifiers:** `[A-Za-z_][A-Za-z0-9_]*`
- **Integers:** decimal literals (no floats in the core value model today)
- **Strings:** `"..."` with backslash escapes
- **Booleans / null:** `true`, `false`, `null`

## Keywords (TypeScript lexer)

Core: `let`, `fn`, `return`, `if`, `else`, `true`, `false`, `null`, `while`, `for`, `in`, `import`, `break`, `continue`, `match`, `struct`, `enum`

Extended / experimental: `async`, `await`, `spawn`, `chan`, `macro`, `rules`, `mut`, `move`, `extern`, `from`, `type`, `ref`, `effect`, `perform`, `handle`, `with`, `resume`, `region`, `reflect`, `alloc`, `derive`, `pub`, `use`

English sugar tokens: `and`, `or`, `not`, `then`, `do`, `end`, `fun`, `var`, `when`, `is`

> **Note:** `type` is a keyword. Prefer the builtin `typeof(x)` when you need a runtime type name as a string. Calling `type(...)` as a function is supported via a special parse path in the TS toolchain.

> **`try` is not a keyword on the TS toolchain.** Result helpers `ok` / `err` / `is_ok` / `is_err` / `unwrap` work; Go-only `try expr` early-return does not.

## Operators

| Class | Operators |
| --- | --- |
| Arithmetic | `+ - * / %` (string `+` concatenates) |
| Comparison | `== != < <= > >=` |
| Logical | `&& \|\| !` (short-circuit); word forms `and` / `or` / `not` (sugar) |
| Index / member | `arr[i]`, `hash["k"]`, `obj.field` |
| Call | `f(a, b)` |
| Pipe | `x \|> f` and `x \|> f(a)` → `f(x)` / `f(x, a)` |
| Path ctor | `Option::Some(x)` |
| Assign | `name = value` (also index assign) |

## Literals

```nex
42
"hello"
true
null
[1, 2, 3]
{ "a": 1, "b": 2 }
```

Hashes also accept identifier keys in many constructor/struct forms.

## English-inspired sugar

Before lexing, `lowerSyntax` rewrites friendly forms into core Nexus. Supported examples:

```nex
set answer to 40
// → let answer = 40;

fun bump(x) do
  return x + 2
end
// → let bump = fn(x) { return x + 2; };

if x > 0 then
  puts("pos")
end
```

Sugar is best-effort. Prefer core syntax in libraries and tests for predictability.

### Design language sugar

```nex
theme dusk {
  ink = "#0f172a"
  accent = "#1d4ed8"
}
// → let dusk = theme({ "ink": "#0f172a", "accent": "#1d4ed8" });

row(style { gap = "space_3" }, [ /* … */ ])
// → row({ "gap": "space_3" }, [ /* … */ ])

view home = page(…)
// → let home = page(…)
```

See [Design language](../design/README.md).

## Semicolons

Statements commonly end with `;`. The parser is tolerant in some positions, but examples and tests use semicolons consistently — follow that style.
