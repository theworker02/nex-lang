# Stdlib (`.nex` modules)

Pure(ish) Nexus modules shipped under `vscode-nexus/stdlib/` (mirrored at repo-root `stdlib/` for the Go CLI).

Import on the tree-walk engine:

```nex
import "strings";
import "result";
```

## `strings`

File: `stdlib/strings.nex`

| Binding | Meaning |
| --- | --- |
| `str_is_empty(s)` | `len(s) == 0` |
| `str_repeat(s, n)` | Repeat `s` `n` times |
| `str_pad_left(s, width, pad)` | Left-pad to `width` using `pad` |

## `result`

File: `stdlib/result.nex`

Helpers around `ok` / `err` Result hashes:

| Binding | Meaning |
| --- | --- |
| `result_map(r, f)` | Map `f` over Ok value; pass Err through |
| `result_unwrap_or(r, fallback)` | Unwrap Ok or return fallback |

## `design`

File: `stdlib/design.nex`

Nexus **Design Language** builders — theme tokens, layout (`stack` / `row` / `grid`), text, brand/nav chrome, and `page` / `page_full` trees. Render with host builtins `design_document` / `design_response` (see [Design language](design/README.md)).

| Binding | Meaning |
| --- | --- |
| `theme` / `nexus_theme` | Token themes |
| `stack` / `row` / `grid` / `hero` / `section` | Layout |
| `headline` / `lead` / `kicker` / `text` | Typography |
| `brand` / `topbar` / `nav` / `link_btn` | Chrome & CTAs |
| `page` / `page_full` | Full document trees |

## Tests

`tests/stdlib_test.nex` exercises strings/result; `scripts/smoke-design.js` covers design builders + sugar. Run via `npm run test:nex` / `npm run smoke`.
## Extending stdlib

1. Add `stdlib/mymodule.nex` that binds helpers with `let`.
2. End the file with a trailing expression (`null` is conventional).
3. Import with `import "mymodule";`.
4. Keep names unique — imports merge into one environment.
