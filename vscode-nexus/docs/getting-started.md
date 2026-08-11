# Getting started

## Requirements

- Node.js 18+ (20+ recommended)
- npm
- Optional: sibling checkout of [`nex-registry`](https://github.com/nex-lang) to run the package website

## Setup

```bash
cd vscode-nexus
npm install
npm run compile
```

The CLI entrypoint is `out/cli.js` after compile.

## Run a program

Tree-walk evaluator (default — full language, modules, host builtins):

```bash
node out/cli.js run examples/modules_demo.nex
# or
npm run run:nex -- examples/modules_demo.nex
```

Bytecode VM (core language only — no `import`, no HTTP host):

```bash
node out/cli.js run examples/vm_demo.nex --vm
```

## REPL

```bash
npm run repl
# or with VM:
node out/cli.js repl --vm
```

## Tests

Discovers `tests/**/*.nex` and `*_test.nex` under the current working directory:

```bash
npm run test:nex
# or specific paths:
node out/cli.js test tests/language_test.nex
```

Assertions: `assert(cond)`, `assert_eq(got, want)`, `assert_eq(got, want, "msg")`.

## Self-hosted path

Runs the user program through the `.nex` lexer/parser/evaluator under `selfhost/`:

```bash
npm run selfhost -- examples/selfhost_demo.nex
# or
node out/cli.js selfhost examples/selfhost_demo.nex
```

See [selfhosting.md](selfhosting.md).

## Smoke checks

```bash
npm run smoke
```

## Editor (VS Code / VSCodium / Open VSX)

**Install:** [Nex LSP on Open VSX](https://open-vsx.org/extension/theworker02/nex-lsp) (`theworker02.nex-lsp`)

From source:

1. Open the `vscode-nexus` folder (or the monorepo) in the editor.
2. Press **F5** (Extension Development Host) or package with `npm run package`.
3. Open a `.nex` file and use **Nexus: Run File** (`Ctrl+Shift+R` / `Cmd+Shift+R`).

Other commands: Run File (Bytecode VM), Run File (Self-hosted), Run Tests, Open REPL, Compile to WASM/LLVM/Native, Publish/Install Package.

## Hello world

Save as `hello.nex`:

```nex
puts("hello, nexus");
42
```

```bash
node out/cli.js run hello.nex
```

Expected: prints `hello, nexus`, then the expression result `42`.
