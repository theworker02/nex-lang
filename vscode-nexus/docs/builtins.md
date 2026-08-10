# Builtins

Nexus builtins come from three layers. Knowing which layer you are on matters for portability and self-hosting.

## 1. Core language builtins (eval + VM)

Defined in `src/language/builtins.ts`. Available in both tree-walk and bytecode engines:

| Name | Role |
| --- | --- |
| `len` | Length of string, array, or hash |
| `puts` | Print inspected values (one line each) |
| `str` | Convert to string via `inspect` |
| `int` | Parse / coerce to integer (`int("")` → `0` on TS) |
| `type` / `typeof` | Runtime type name |
| `push` | Append to array (returns new array) |
| `first` / `last` / `rest` | Array accessors |
| `keys` | Hash keys |
| `has` / `get` | Hash membership / lookup |
| `split` / `join` / `trim` / `lower` / `upper` | Strings |
| `contains` / `starts_with` / `replace` / `slice` | Strings / arrays |
| `ok` / `err` / `is_ok` / `is_err` / `unwrap` | Result hashes |
| `map` / `filter` | Higher-order over arrays |
| `assert` / `assert_eq` | Testing |
| `getenv` | Environment variable (string or null-ish) |
| `escape_html` | HTML-escape a string |
| `merge` | Shallow-merge one or more hashes (later keys win) |

## 2. Stdlib install (tree-walk host)

Registered by `installStdlib` when evaluating (not the minimal VM path):

### Filesystem (`std/fs`)

- `fs_read(path)` → string
- `fs_write(path, contents)` → byte/char count
- `fs_exists(path)` → bool
- `fs_list(dir)` → newline-joined names

### Crypto (`std/crypto`)

- `sha256`, `sha512`, `md5` (hex digests)
- Additional helpers as implemented in `src/std/crypto.ts`

### Net (`std/net`)

- `net_fetch(url [, timeout_ms])` — blocking HTTP(S) GET helper

### Task (`std/task`)

- `spawn_task`, `task_yield`, `worker_hash`

### Memory / FFI

- `mem_stats`, `mem_collect`
- `ffi_load`, `ffi_symbol`, `ffi_call`, `ffi_available` (optional `koffi`)

## 3. Web / registry host builtins

Installed only when a `WebHost` is attached (CLI `run` for apps that register routes, or `npm run registry`). Implemented in TypeScript — **not** pure `.nex`.

### HTTP routing & responses

- `http_get` / `http_post` / `http_put` / `http_patch` / `http_delete` / `http_not_found`
- `json`, `html`, `html_doc`, `redirect`, `file_response`
- `design_document`, `design_response`, `design_render`, `design_css` — Nexus Design Language render (see [design/](design/README.md))
- `with_cookie`, `clear_cookie`

### Config & data

- `env`, `config`, `path_join`
- `json_parse`, `json_stringify`, `toml_parse`
- `markdown_html`, `docs_get`
- `re_match`, `multipart_text`, `multipart_file`
- `send_email` (stub/limited depending on env)

### Crypto (host)

- `sha256`, `sha256_bytes`, `bcrypt_hash`, `bcrypt_check`, `random_hex`, `gravatar_url`

### Demo DB (`db_*`)

Browse/search helpers backed by an **in-memory** store seeded from registry `storage/` when present:

- Counts: `db_count_packages`, `db_count_versions`, `db_count_users`, `db_sum_downloads`
- Lists: `db_list_recent`, `db_list_popular`, `db_search`, `db_get_package`, …
- Many auth/publish/admin APIs exist as **stubs** that return errors or empty data on the TS demo host

Full Postgres-backed publish/auth remains a Go-host strength when `DATABASE_URL` is set.

## Portability rules of thumb

| You need… | Use |
| --- | --- |
| Core algorithms, tests | Core builtins only |
| Local files inside selfhost | Host still injects `fs_read` / `puts` |
| Registry website | Web host builtins + `import` + eval |
| Bytecode VM | Core builtins only; no `import` |
