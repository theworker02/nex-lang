# Nexus Language Specification (implemented)

This document describes **behavior that exists today**, with the **TypeScript toolchain** (`vscode-nexus/`) as the primary reference. The legacy Go CLI (`cmd/nex` + `pkg/*`) is noted where it differs.

For the full docs tree, see [vscode-nexus/docs/README.md](../vscode-nexus/docs/README.md).

## Engines

| Engine | Command (TS) | Command (Go) | Status |
|--------|--------------|--------------|--------|
| Tree-walk evaluator | `node out/cli.js run file.nex` | `nex run file.nex` | Full language + host builtins + `import` |
| Bytecode VM | `… run file.nex --vm` | `nex run --vm file.nex` | Core language (no `import`, no host HTTP/DB) |
| Self-host | `… selfhost file.nex` | — | Subset evaluator written in `.nex` |
| REPL | `… repl` / `repl --vm` | `nex repl` | Interactive |
| WASM / LLVM | Editor compile commands | — | Core subset codegen (TS) |

## Lexical structure

- Comments: `//` to end of line
- Identifiers: `[A-Za-z_][A-Za-z0-9_]*`
- Integers: decimal literals
- Strings: `"..."` with `\` escapes
- Core keywords: `let`, `fn`, `return`, `if`, `else`, `true`, `false`, `null`, `while`, `for`, `in`, `import`, `break`, `continue`, `match`, `struct`, `enum`
- TS also recognizes experimental keywords (`async`, `effect`, `region`, …) and English sugar (`fun`, `do`/`end`, `when`, …)

### Keyword differences

| Feature | TypeScript | Go |
| --- | --- | --- |
| `try expr` Result early-return | **Not implemented** | Implemented |
| `enum` / `::` constructors | Implemented | Limited / evolving |
| `typeof` builtin | Preferred (`type` is keyword) | `type` builtin common |

## Types (gradual)

Optional annotations on `let` and parameters: `int`, `string`, `bool`, `array`, `hash`, `fn`, `any` (and aliases). Annotations are checked dynamically at bind/call time; they are not a static type system yet.

## Declarations & control flow

```nex
let x = 1;
let y: int = 2;
struct Point { x, y };
let add = fn(a: int, b: int) -> int { return a + b; };

if (cond) { ... } else { ... };
while (cond) { break; continue; };
```

## Expressions

- Arithmetic: `+ - * / %` (string `+` concatenates)
- Comparison: `== != < <= > >=`
- Logical: `&& || !` (short-circuit)
- Indexing: `arr[i]`, `hash["k"]`, `str[i]`
- Members: `obj.field` (hashes / structs)
- Calls: `f(a, b)`
- Pipes: `x |> f` and `x |> f(a)` → `f(x)` / `f(x, a)`
- Match: `match (v) { 0 -> ..., _ -> ... }` (also `=>`)
- Results: `ok(v)`, `err(e)`, `is_ok`, `is_err`, `unwrap`
- Go-only: `try expr` unwraps a Result or early-returns the Err from the enclosing function

Structs compile to constructor callables that produce hashes (often with an `__struct` tag).

## Modules

```nex
import "helper.nex";     // relative to current file, then app root
import "demo";           // .modules/demo/mod.nex or main.nex or demo.nex
import "strings";        // stdlib/strings.nex
```

Resolution order (tree-walk):

1. Absolute path (if given)
2. Relative to the importing file (`__dir__`)
3. Relative to the app / workspace root
4. `.modules/<path>` (package installs)
5. Language `stdlib/`

Imports evaluate into the **shared environment**. Re-imports of the same resolved path are no-ops.

The bytecode VM does not support `import` — use the tree-walk engine for modular programs.

## Core builtins

`len`, `puts`, `str`, `int`, `type` / `typeof`, `push`, `first`, `last`, `rest`, `keys`, `has`, `get`, `contains`, `split`, `join`, `trim`, `lower`, `upper`, `starts_with`, `replace`, `slice`, `map`, `filter`, `ok`, `err`, `is_ok`, `is_err`, `unwrap`, `assert`, `assert_eq`, `getenv`.

Host builtins (tree-walk + host attached): `http_*`, `db_*`, `fs_*`, JSON/TOML, crypto, templates — see [vscode-nexus/docs/builtins.md](../vscode-nexus/docs/builtins.md).

## Testing

```text
# TypeScript
cd vscode-nexus && npm run test:nex

# Go
nex test
nex test tests/foo.nex
```

Use `assert(cond)` / `assert_eq(got, want)` / `assert_eq(got, want, "msg")`.

## Package layout (recommended)

```text
nexus.toml          # package manifest
src/ or app/        # application .nex sources
stdlib/             # language stdlib
.modules/           # installed dependencies
tests/              # *_test.nex or any .nex under tests/
examples/           # demos
```

## Bytecode VM surface

Supported: literals, lets/assigns, arithmetic/logic, if/else, while/break/continue, functions/closures, arrays/hashes/index/member, builtins, structs, match, pipes (as implemented).

Not supported: `import`, host HTTP/DB builtins, self-host pipeline.

## Self-host subset

See [vscode-nexus/docs/selfhosting.md](../vscode-nexus/docs/selfhosting.md). Smaller than the TS evaluator; host still supplies `fs_read` / `puts` / `__argv__`.
