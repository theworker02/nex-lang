# Self-hosting

Nexus can run a **self-hosted** pipeline written in `.nex`, bootstrapped by the TypeScript host.

Also see [`../selfhost/README.md`](../selfhost/README.md).

## Architecture

```
User .nex program
       │
       ▼
┌──────────────────┐
│  TS host (cli)   │  reads selfhost/main.nex, injects __argv__, fs_read, puts
└────────┬─────────┘
         │ evaluates (tree-walk)
         ▼
┌──────────────────┐
│  selfhost/*.nex  │  lex → parse → eval  (all Nexus)
└────────┬─────────┘
         │ interprets
         ▼
   program result / puts output
```

## Commands

```bash
cd vscode-nexus
npm run compile
node out/cli.js selfhost examples/selfhost_demo.nex
npm run selfhost -- examples/selfhost_demo.nex
```

Editor: **Nexus: Run File (Self-hosted)** (`nexus.selfhost`).

## Supported subset (self-hosted evaluator)

- `let` / assignment / `return`
- `fn` literals and calls
- `if` / `else`, `while`
- integers, strings, booleans, `null`
- arithmetic `+ - * / %`, comparisons, prefix `!` `-`
- arrays + index, hashes + member/index
- `match` with literal / `_` / ident patterns
- builtins: `puts`, `len`, `str`, `int`, `typeof`, `push`, `first`, `last`, `rest`, `slice`, `keys`, `has`, `get`, `assert`, `assert_eq`

Use `typeof(x)` instead of `type(x)` — `type` is a keyword in the host grammar.

## Not yet in the self-hosted evaluator

Available on the TS host path, but **not** in `selfhost/evaluator.nex` today:

- `import` / modules
- `struct` / `enum` / path constructors
- pipes `|>`
- `ok` / `err` / Result helpers (beyond what you reimplement)
- macros, effects, async/await, regions
- VM bytecode emission
- web host builtins

## What the host still provides

| Capability | Why |
| --- | --- |
| Loading `selfhost/main.nex` | Bootstrap entry |
| `__argv__` | CLI file path / args |
| `fs_read` | Read user source |
| `puts` / core builtins | I/O shared with host |

A language always needs *some* native substrate. Self-hosting shrinks that substrate toward a thin loader + FFI; it does not eliminate it.

## File tree

```
vscode-nexus/selfhost/
  lexer.nex       tokenize source
  parser.nex      recursive-descent AST (hash nodes)
  evaluator.nex   tree-walk interpreter
  main.nex        CLI driver
  nexc.nex        alias entry
```

## Next bootstrap steps

1. Expand the subset (structs, pipes, in-nex `import`).
2. Emit a stable AST or bytecode format from the `.nex` compiler.
3. Shrink the TS surface toward a thin loader + host FFI once `selfhost/` can run itself more completely.
