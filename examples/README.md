# Examples

<p align="center">
  <img src="../assets/logo.png" alt="Nexus" width="72" height="72">
</p>

Canonical demos for the TypeScript toolchain live under [`vscode-nexus/examples/`](../vscode-nexus/examples/) — see the [examples index](../vscode-nexus/docs/examples.md).

This directory mirrors several demos for the legacy Go CLI and shared layouts:

| File | Notes |
| --- | --- |
| `hello.nex` | Minimal |
| `features.nex` | Rich demo; uses Go-only `try` |
| `modules_demo.nex` | Stdlib import |
| `vm_demo.nex` | Bytecode-friendly |
| `selfhost_demo.nex` | Self-host subset |

Prefer:

```bash
cd vscode-nexus
node out/cli.js run examples/modules_demo.nex
```
