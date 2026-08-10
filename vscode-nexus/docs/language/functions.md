# Functions

## Function literals

```nex
let add = fn(a, b) {
  return a + b;
};

puts(add(20, 22));
```

With gradual types:

```nex
let add = fn(a: int, b: int) -> int {
  return a + b;
};
```

Functions are first-class values and close over their environment.

## Calls

```nex
f(1, 2)
obj.method(x)   // if `method` is a function field
```

## Pipes

```nex
assert_eq([1, 2, 3] |> map(fn(x) { x + 1 }), [2, 3, 4]);
puts([1, 2, 3, 4, 5] |> filter(fn(x) { x % 2 == 1 }));
```

- `x |> f` → `f(x)`
- `x |> f(a)` → `f(x, a)` (piped value is the **first** argument)

`map` and `filter` are core builtins (require a function applicator from the host/evaluator).

## English sugar

```nex
fun bump(x) do
  return x + 2
end
```

Lowers to a `let bump = fn(x) { ... };` form.

## Async / spawn (experimental)

Keywords `async`, `await`, `spawn`, and `chan` parse. Runtime support is partial via the TS async evaluator and `std/task` helpers (`spawn_task`, `task_yield`, `worker_hash`). Do not assume Go-style green threads or CSP channels are production-ready.

Prefer synchronous `fn` for portable Nexus code.
