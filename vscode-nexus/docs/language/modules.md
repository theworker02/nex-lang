# Modules

Modules work on the **tree-walk** engine only. The bytecode VM rejects or does not load `import` — use `eval` (default `run`) when modules are required.

## Syntax

```nex
import "strings";
import "result";
import "helper.nex";
import "demo";
```

Imports evaluate into a **shared environment** (bindings become visible in the importer). Re-importing the same resolved path is a no-op.

There is **no** selective `export` / `import { x }` yet. Name clashes are possible across packages — choose unique names.

## Resolution order

1. Absolute path (if given)
2. Relative to the importing file (`__dir__`)
3. Workspace / app root
4. `.modules/<name>` (installed packages)
5. `stdlib/` (extension `stdlib/` or repo-root `stdlib/`)

Candidates for a bare name `foo`:

- `foo.nex`
- `foo/mod.nex`
- `foo/main.nex`

## Stdlib modules

Shipped with the toolchain:

| Import | File | Provides |
| --- | --- | --- |
| `"strings"` | `stdlib/strings.nex` | `str_is_empty`, `str_repeat`, `str_pad_left` |
| `"result"` | `stdlib/result.nex` | `result_map`, `result_unwrap_or` |

Example:

```nex
import "strings";

puts(str_repeat("nex ", 3));
puts(str_pad_left("42", 5, "0"));
```

## Package installs

`RegistryClient.install` materializes archives under `.modules/`. After install, `import "pkgname"` resolves from `.modules/` per the order above. See [packages.md](../packages.md).
