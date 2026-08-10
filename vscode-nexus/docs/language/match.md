# Match, structs, and enums

## `match`

```nex
let classified = match (4) {
  0 -> 0,
  1 -> 1,
  _ -> 16
};
```

Arms accept **`->` or `=>`**. Patterns:

- Integer / string / bool literals
- `_` wildcard
- Identifier bindings
- Constructor / path patterns (`Option::Some(n)`)

Parentheses around the scrutinee are optional: `match (x)` and `match x` both parse.

## Structs

```nex
struct Point { x, y };
let p = Point(3, 4);
assert_eq(p.x + p.y, 7);
```

Field types may be annotated in declarations (`x: int`) for documentation and reflection experiments; runtime remains gradual.

## Enums

```nex
enum Option {
  Some(x),
  None
}

let v = Option::Some(42);
match v {
  Option::Some(n) => { puts(n) },
  Option::None => { puts("none") }
}
```

## Reflection / derive (experimental)

Attributes such as `#derive(json) struct Foo` and `#reflect(Type)` are lowered/expanded by the pipeline. Example demos: `examples/ultimate.nex`, `examples/effects-regions.nex`. Treat generated helpers (`Point_fields`, `Point_to_json`) as experimental.
