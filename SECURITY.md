# Security Policy

## Supported versions

Security fixes are applied to the **current `main` branch** of [theworker02/nex-lang](https://github.com/theworker02/nex-lang). There is no long-term support (LTS) line yet.

| Component | Supported |
| --- | --- |
| TypeScript toolchain (`vscode-nexus/`, `nex-ts` CLI) | Yes — `main` |
| Nex LSP extension (`theworker02.nex-lsp`) | Yes — latest release / `main` |
| Legacy Go CLI (`cmd/nex`, `pkg/*`) | Best-effort only |
| Optional local `nex-registry` sibling | Out of scope for this repository (not published here) |

Older tags and forks are unsupported unless noted in a release.

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security-sensitive reports.

Preferred options (pick one):

1. **GitHub private vulnerability reporting** — use **Security → Report a vulnerability** on [theworker02/nex-lang](https://github.com/theworker02/nex-lang/security/advisories/new) if enabled for the repo.
2. **GitHub discussion with maintainers** — open a [private security advisory](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability) or contact maintainers via GitHub (@theworker02) and ask for a private channel.

Include, when possible:

- Affected component (CLI, extension, host builtins, design site, etc.)
- Version / commit hash
- Steps to reproduce
- Impact (code execution, path traversal, data exposure, etc.)
- Whether a public PoC already exists

## What to expect

| Stage | Expectation |
| --- | --- |
| Acknowledgement | Within **7 days** (best effort; this is a small open-source project) |
| Status update | When we have a triage decision or fix plan |
| Fix / disclosure | Coordinated when practical; we may publish a GitHub Security Advisory |

We will not ask for payment for reports. We appreciate responsible disclosure.

## Scope notes (honest)

- The Nexus **CLI and extension run locally** on the user’s machine. Treat untrusted `.nex` programs like untrusted scripts: filesystem, network, and other host builtins can do real I/O when enabled.
- The **GitHub Pages language site** is static HTML (no server-side app, no account system).
- **Open VSX** and other marketplaces are third-party platforms with their own policies.
- This policy covers the software in **this repository** only, maintained by **theworker02 / nex-lang project maintainers** — not a separate legal entity.

## Safe harbor

We will not pursue legal action against researchers who:

- Act in good faith and avoid privacy violations, destruction of data, and service disruption
- Give us a reasonable chance to fix issues before public disclosure
- Do not exploit findings beyond what is needed to demonstrate the issue
