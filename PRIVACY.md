# Privacy Policy

**Last updated:** 2026-08-10

This policy describes how the **Nexus** open-source project ([theworker02/nex-lang](https://github.com/theworker02/nex-lang)) handles information. Maintainers: **theworker02 / nex-lang project maintainers**. This is not a commercial product offered by a registered company.

## Summary

- The **CLI and editor extension** run on your machine. We do not operate a Nexus cloud backend that collects your source code or personal data.
- The **public language website** on GitHub Pages is a **static** site. We do not add first-party analytics or tracking pixels in this repository’s site build.
- We **do not sell** personal data.
- Third parties you choose to use (GitHub, Open VSX, your editor, your OS) have their own policies.

## What this software does locally

### Nexus CLI (`nex-ts` / `vscode-nexus`)

When you run programs, the REPL, tests, or the local language site (`npm run site`), processing happens **on your computer**. Files you open, compile, or serve are not sent to the Nexus maintainers by the toolchain itself.

Host builtins (filesystem, HTTP client/server, crypto, optional database adapters, etc.) can access resources you configure. That access is under **your** control and environment — not a Nexus telemetry pipeline.

### Nex LSP editor extension

The extension provides language features in VS Code / VSCodium / compatible editors. It does not phone home to a Nexus analytics service. Editor crash reports, marketplace stats, or sync features (if any) are governed by **Microsoft / Open VSX / your editor vendor**, not by this policy.

### Optional local package registry

A private sibling `nex-registry` checkout (not published with this repo) may store package metadata and artifacts **locally** or wherever you point its configuration. That is your deployment.

## Public website (GitHub Pages)

URL: [https://theworker02.github.io/nex-lang/](https://theworker02.github.io/nex-lang/)

- Content is static HTML/CSS/assets produced by `npm run build:site`.
- **No first-party cookies, accounts, or analytics** are implemented in the site sources in this repository.
- Hosting is provided by **GitHub Pages**. GitHub may collect standard web server / CDN logs (IP address, user agent, etc.) under [GitHub’s Privacy Statement](https://docs.github.com/en/site-policy/privacy-policies/github-privacy-statement). We do not receive a separate marketing profile from that traffic beyond what GitHub exposes to repository owners (if anything).

## Third-party services

| Service | Role | Notes |
| --- | --- | --- |
| [GitHub](https://github.com/) | Source hosting, Issues, Discussions, Actions, Pages | Subject to GitHub’s terms and privacy policy |
| [Open VSX](https://open-vsx.org/) | Optional extension marketplace distribution | Third-party; see Open VSX / Eclipse Foundation policies |
| Your package registries, DBs, or APIs | Only if you configure them | Outside this project’s control |

Installing **Nex LSP** from Open VSX (or elsewhere) may involve download counts and publisher metadata on that marketplace. That is not personal data sold by the nex-lang maintainers.

## Personal data we might see as maintainers

If you contact us or contribute via GitHub, we may see:

- Your GitHub username, avatar, and public profile
- Email addresses you include in commits, issues, or security reports
- Content of bug reports, PRs, and discussions

We use that information only to maintain the project (triage, review, security response). We do not sell it or use it for advertising.

## Children

The project is not directed at children under 13. Do not submit personal information of children through project channels.

## Changes

We may update this file in the repository. The “Last updated” date will change when we do. Continued use of the software and site after updates constitutes awareness of the revised policy for open-source purposes.

## Contact

- GitHub: [@theworker02](https://github.com/theworker02)
- Issues / security: see [SUPPORT.md](SUPPORT.md) and [SECURITY.md](SECURITY.md)

Site mirror: [Privacy](https://theworker02.github.io/nex-lang/privacy/)
