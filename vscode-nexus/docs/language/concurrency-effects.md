# Concurrency, effects, and regions

These features exist in the TypeScript grammar and parts of the runtime, but they are **not** the stable core. Prefer them only for experiments.

## Concurrency

| Feature | Status |
| --- | --- |
| `async` / `await` expressions | Parsed; evaluator has async paths |
| `spawn` | Parsed; limited runtime |
| `chan(...)` | Parsed; not a full CSP system |
| `spawn_task` / `task_yield` builtins | Host helpers in `std/task` |
| `worker_hash` | Offloads SHA-256 to a worker thread |

Honest guidance: write synchronous scripts and HTTP handlers unless you are extending the runtime itself.

## Algebraic effects

```nex
effect Log {
  info(msg)
}

handle {
  perform Log::info("hello from effect")
} with Log {
  info(msg) => { puts(msg); resume(0) }
}
```

`effect` / `perform` / `handle` / `with` / `resume` parse and pass through effect diagnostics when enabled. Demo: `examples/effects-regions.nex`. CLI `run` currently sets `enableEffects: false` for the default path — editor diagnostics may still surface effect issues when configured.

## Regions

```nex
region scratch {
  let p = Point_to_json(0)
  puts(p)
}
```

Region inference annotates lexical arenas for experimental memory accounting (`mem_stats`, `mem_collect`). This is not a replacement for a GC story; the host still owns most allocations.

## Macros

`macro` / `rules` and `name!(...)` invocations exist in the parser and macro expander. Coverage is incomplete — keep macros out of production packages for now.
