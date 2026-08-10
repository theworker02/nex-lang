# Types

Nexus uses **gradual typing**: annotations are optional and checked dynamically when present.

## Annotations

Allowed names (and aliases): `int`, `string`, `bool`, `array`, `hash`, `fn`, `any`.

```nex
let x: int = 1;
let add = fn(a: int, b: int) -> int {
  return a + b;
};
```

Return annotations (`-> int`) are recorded; enforcement is primarily at bind/call sites for parameters and `let` bindings.

## Runtime values

| Tag | Examples |
| --- | --- |
| INTEGER | `1`, `-3` |
| STRING | `"nex"` |
| BOOLEAN | `true` / `false` |
| NULL | `null` |
| ARRAY | `[1, 2]` |
| HASH | `{ "k": 1 }`, struct instances |
| FUNCTION | `fn(...) { ... }` |
| BUILTIN | host/core builtins |
| ERROR | runtime failures |
| Result-like hash | `{ ok, value, error }` from `ok`/`err` |

Inspect with:

```nex
puts(typeof(42));   // preferred
puts(type(42));     // also available on TS host
```

## Structs

```nex
struct Point { x, y };
let p = Point(3, 4);
puts(p.x + p.y);
```

Structs compile to constructor callables that produce hashes (often tagged with `__struct`).

## Enums / ADTs

```nex
enum Option {
  Some(x),
  None
}

let v = Option::Some(41);
match v {
  Option::Some(n) => { puts(n) },
  Option::None => { puts("none") }
}
```

Both `->` and `=>` are accepted after match patterns.

## Results (no `try` on TS)

```nex
let doubled = fn(n) {
  if (n > 0) {
    return ok(n * 2);
  };
  return err("non-positive");
};

assert(is_ok(doubled(21)));
assert_eq(unwrap(doubled(21)), 42);
assert(is_err(doubled(0)));
```

On the Go CLI, `try expr` can early-return an `Err` Result from the enclosing function. That keyword is **not** implemented in the TypeScript lexer/evaluator yet — unwrap explicitly or branch on `is_ok` / `is_err`.

## Ownership / mutability keywords

`mut`, `move`, `ref`, `&` / `&mut` appear in the grammar and ownership checker. Treat ownership as **experimental diagnostics**, not a finished borrow checker. CLI `run` disables ownership hard-fail by default (`checkOwnership: false`).
