# Examples index

All paths relative to `vscode-nexus/` unless noted.

| File | Engine | What it shows |
| --- | --- | --- |
| `examples/modules_demo.nex` | eval | `import "strings"` + stdlib helpers |
| `examples/vm_demo.nex` | vm | Functions, arrays, `puts` without imports |
| `examples/selfhost_demo.nex` | selfhost | Subset program for `selfhost` CLI |
| `examples/ultimate.nex` | eval | Sugar, enums, `sha256`, `mem_stats` |
| `examples/site/main.nex` | eval + host | Design language self-site (`npm run site`) |
| `examples/effects-regions.nex` | eval (experimental) | Effects, regions, reflection helpers |

Repo-root mirrors (for Go CLI / shared layout):

| File | Notes |
| --- | --- |
| `examples/hello.nex` | Minimal hello |
| `examples/features.nex` | Richer demo; uses Go-only `try` — prefer TS `language_test.nex` patterns on Node |
| `examples/modules_demo.nex` | Same idea as vscode-nexus copy |
| `examples/vm_demo.nex` | VM-friendly |
| `examples/selfhost_demo.nex` | Selfhost smoke |

## Tests as examples

| File | Focus |
| --- | --- |
| `tests/language_test.nex` | Arithmetic, structs, match, Results, pipes |
| `tests/stdlib_test.nex` | `strings` + `result` modules |

```bash
npm run test:nex
node out/cli.js run examples/modules_demo.nex
node out/cli.js run examples/vm_demo.nex --vm
node out/cli.js selfhost examples/selfhost_demo.nex
```
