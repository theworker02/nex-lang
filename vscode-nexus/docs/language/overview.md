# Language overview

Nexus (`.nex`) is a small, gradual-typed scripting language with a tree-walk runtime, an optional bytecode VM, and a growing self-hosted pipeline. The **canonical implementation** for day-to-day use is the TypeScript toolchain in `vscode-nexus/`.

## Design goals (honest)

| Goal | Status |
| --- | --- |
| Friendly scripting surface | Done — `let`, `fn`, arrays, hashes, `match` |
| Gradual types | Partial — runtime checks on annotations |
| Modules | Done on tree-walk; not on bytecode VM |
| Host I/O (FS, HTTP, crypto) | Done via **TypeScript builtins**, not pure `.nex` |
| Self-hosting | Working subset — see [selfhosting.md](../selfhosting.md) |
| Package registry apps | Runnable on TS host with in-memory demo DB |
| Design language (theme/layout → HTML/CSS) | Done — `stdlib/design.nex` + host render; `/design` self-site |
| Static type system | Not implemented |
| Full engine parity (eval ↔ VM ↔ WASM) | Not yet |

## Engines at a glance

| Engine | Command | Modules | Host HTTP/DB | Notes |
| --- | --- | --- | --- | --- |
| Tree-walk `eval` | `run file.nex` | Yes | Yes | Default; registry apps |
| Bytecode `vm` | `run file.nex --vm` | No | No | Core language + core builtins |
| Self-host | `selfhost file.nex` | No | Via host `fs_read`/`puts` | Subset evaluator in `.nex` |
| WASM / LLVM | Editor compile commands | N/A | N/A | Codegen for a **core** subset; not a full runtime |

## What a Nexus program looks like

```nex
import "strings";

let greet = fn(name: string) {
  return "hello, " + name;
};

puts(greet("nexus"));
puts(str_repeat("!", 3));

let classified = match (2) {
  0 -> "zero",
  1 -> "one",
  _ -> "many"
};
puts(classified);
```

## Host vs language

Many powerful APIs (`http_get`, `db_*`, `fs_read`, `sha256`, templates) are **host-provided builtins** implemented in TypeScript (or Go on the legacy CLI). They are part of the practical language surface for apps, but they are not pure Nexus. Documented separately in [builtins.md](../builtins.md).

## Go CLI differences

The legacy Go interpreter still supports some features the TS path does not yet, notably:

- `try expr` Result early-return
- Postgres-backed registry host when `DATABASE_URL` is set

Prefer TS docs for new work; treat Go as legacy/compat unless you need those paths.
