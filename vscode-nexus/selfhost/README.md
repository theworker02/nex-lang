# Self-hosting Nexus

<p align="center">
  <img src="../media/logo.png" alt="Nexus" width="96" height="96">
</p>

Nexus can now run a **self-hosted** pipeline written in `.nex`, bootstrapped by the TypeScript host under `vscode-nexus/`.

Extended docs: [../docs/selfhosting.md](../docs/selfhosting.md) · [../docs/README.md](../docs/README.md)

## Architecture

```
User .nex program
       │
       ▼
┌──────────────────┐
│  TS host (cli)   │  reads selfhost/main.nex, injects __argv__, fs_read, puts
└────────┬─────────┘
         │ evaluates
         ▼
┌──────────────────┐
│  selfhost/*.nex  │  lex → parse → eval  (all Nexus)
└────────┬─────────┘
         │ interprets
         ▼
   program result / puts output
```

## Subset (self-hosted evaluator)

Supported today:

- `let` / assignment / `return`
- `fn` literals and calls
- `if` / `else`, `while`
- integers, strings, booleans, `null`
- arithmetic `+ - * / %`, comparisons, prefix `!` `-`
- arrays + index, hashes + member/index
- `match` with literal / `_` / ident patterns
- builtins: `puts`, `len`, `str`, `int`, `typeof`, `push`, `first`, `last`, `rest`, `slice`, `keys`, `has`, `get`, `assert`, `assert_eq`

Use `typeof(x)` instead of `type(x)` — `type` is a keyword in the host grammar.

Not yet in the self-hosted evaluator (host TS path still has them): `import`, structs, pipes, `try`/`ok`/`err`, macros, effects, async, VM bytecode.

## Host still provides

| Capability | Why |
| --- | --- |
| Loading `selfhost/main.nex` | Bootstrap entry |
| `__argv__` | CLI file path / args |
| `fs_read` | Read user source |
| `puts` / core builtins | I/O and helpers called by both host and selfhost builtins |

## File tree

```
vscode-nexus/selfhost/
  lexer.nex       tokenize source
  parser.nex      recursive-descent AST (hash nodes)
  evaluator.nex   tree-walk interpreter
  main.nex        CLI driver
  nexc.nex        alias entry
```

## Commands (no Go)

```bash
cd vscode-nexus
npm run compile
node out/cli.js selfhost examples/selfhost_demo.nex
# or
npm run selfhost -- examples/selfhost_demo.nex
```

In the editor: **Nexus: Run File (Self-hosted)** (`nexus.selfhost`).

## Next bootstrap step

1. Expand the subset (structs, pipes, `import` resolution in-nex).
2. Emit a stable AST or bytecode format from the `.nex` compiler.
3. Shrink the TS surface toward a thin loader + host FFI once the self-hosted path covers the language used by `selfhost/` itself.
