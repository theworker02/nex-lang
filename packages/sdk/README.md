# `@theworker02/nex-sdk`

Typed control client for the Nexus package registry REST API. Mirrors the Go client in `pkg/client`.

```ts
import { NexClient } from "@theworker02/nex-sdk";

const api = new NexClient({
  baseUrl: process.env.NEX_REGISTRY_URL ?? "http://localhost:8080",
  token: process.env.NEX_API_KEY,
});

await api.login("you", "password");
const latest = await api.resolvePackage("example");
const bytes = await api.downloadPackage("example", latest.version);

await api.publish({
  manifest: new Blob([nexusToml], { type: "text/plain" }),
  package: new Blob([nexBytes]),
});
await api.yank("example", latest.version, "broken checksum");
```

## Methods

| Method | Endpoint |
| --- | --- |
| `login(login, password)` | `POST /api/auth/login` |
| `profile()` | `GET /api/user/profile` |
| `createApiKey(name?)` | `POST /api/user/api-keys` |
| `resolvePackage(name, version?)` | `GET /api/v1/packages/...` |
| `downloadPackage(name, version)` | `GET /api/v1/packages/.../download` |
| `publish({ manifest, package, readme? })` | `POST /api/v1/publish` |
| `yank(name, version, reason)` | `POST /api/v1/packages/.../yank` |
