# gourl internal — Go Backend Conventions

Subdirectory conventions for the Go backend packages. Root-level conventions (commits, branch model, versioning) live in the repository-root `CLAUDE.md` and apply here too.

## Commands

- Build/run: `go build ./cmd/gourl` / `go run ./cmd/gourl` — **build fails without the gitignored go:embed files** `internal/webui/dist` + `internal/webui/icon.svg`; run `powershell -File scripts/build-frontend.ps1` (repo root) first
- Test: `go test ./...` (miniredis + `:memory:`, no real Redis; CI builds the frontend embed before testing)
- Vet: `go vet ./...` — must stay clean
- Defaults: `./config/config.yaml`, `./data/gourl.db`, `./data/log`; env overrides live in cmd/gourl/main.go

## Principles

- stdlib `net/http` only (Go 1.22+ patterns, `{code...}` wildcards); gofmt + vet clean
- Log in English via `slog` only, never `log.Printf`; all user-facing strings bilingual zh/en — API error `code`s are stable English identifiers the frontend maps
- SQLite stores links and related records only; business config in `config.yaml`; runtime/secrets in env vars

## Package conventions

### config

- `Manager.Update` writes back atomically and hot-swaps (the settings page relies on it)
- **`Get()` normalizes slice fields to empty (not nil) slices — JSON must never emit `null` arrays** (the frontend crashed on `null.join()` once)
- Three fields are `json:"-"`: `password_hash`, `session_epoch`, `webui_enabled` — never expose them, and `updateConfig` must carry all three over from the live config or a PUT wipes/resets them

### store

- Schema changes go through the ordered `migrations` slice (details in code comments) — never raw DDL
- **Soft deletes** (links + tokens): only `deleted = 1`; a partial index keeps codes unique among non-deleted rows. Only db-export surfaces deleted rows
- `daily_clicks` is permanent history — never clean up click rows on link deletion; `ApplyCounts` resolves the live id per flush so a reused code counts from zero
- `backups` is append-only: pre-edit snapshots with `b-1, b-2, …` ids; the api layer backs up on manual edits, renames and batch conflict=update, never on count flushes
- `DeleteLinks`/`DeleteExpired` return the first link actually deleted so api logs can name a concrete `code` + `id`
- **`GetLink` serves from an in-memory TTL cache (60s)**, totals included — every write path must invalidate: create/update/rename/meta/delete del the code, `ApplyCounts` dels touched codes, `DeleteExpired` clears the whole cache
- `:memory:` SQLite is per-connection — fine for tests; a shared in-memory DB across goroutines needs a file DSN

### api

- Target URLs accept **any scheme** (only `base_url`/`extra_base_urls` stay http(s)); non-http(s) targets skip title fetching
- Multi-level codes come from `PathValue` with a possible leading slash — always route through `pathCode()`
- `site.description`/`site.keywords` are injected into every served page: SPA shell at serve time (`spaIndex`/`seoMeta`) and 404/403 templates
- **`ip_blocks` is the outermost middleware** (every route incl. health); the `cors` middleware sits inside it, so bans cover preflights
- CORS: `Access-Control-Allow-Origin: *` on `/api/` responses — safe only because auth is Bearer tokens, no cookies
- Auth resolves per request via `Server.adminAuth()` — hash/epoch changes apply without a restart; legacy `ADMIN_PASSWORD` migrates once into the config file
- Sessions are stateless `exp.epoch.nonce.hmac`; TTL applies at issue time only; the `epoch` claim must match `config.session_epoch`; `SESSION_SECRET` unset → ephemeral per-process secret (sessions do not survive a restart)
- `requireAuth` stamps the request context with `actor` (session|token|app) — bearer requests whose UA starts with `gourl/<version>` become `actor=app`; business-event logs must go through `logInfo`/`logWarn`
- **`GET /api/v1/config` doubles as the app's connection probe** (polled every 10s + on app foreground) — keep it behind `requireAuth`, never public, or the probe silently reports the wrong state. The one-shot connect probe is the public `GET /api/v1/auth/status`: it returns `configured` + a real credential check (`authenticated` + `actor` session|token|app, mirroring requireAuth's order) so bad tokens are rejected before the app saves them
- The SQL console (`POST /api/v1/db`) runs admin SQL in one transaction, gated on `sql_console_enabled` (default off, togglable only via config file + `gourl reload`)
- New API endpoints must update `internal/webui/openapi.yaml` too — CI lints the spec with Redocly (`openapi` job), so structural errors fail the build

### fetcher

- **Any reachable host is allowed** (no SSRF filtering — fetches are only triggered by an authenticated admin); 5s timeout, 5-hop redirect cap and 1 MiB body limit stay
- Lenient parsing: non-200/non-html still parsed for `<title>`; failures log at warning level. `store.UpdateMeta` guards both fields in SQL: an empty fetched title never wipes the old one, and a stored description is **never overwritten once non-empty** (user-entered descriptions survive every refetch). Any link mutation re-enqueues a fetch except patches that carry an explicit title

### shortcode

- `builtinReserved` prefixes (`api`, `admin`, `docs`, …) — **new system routes MUST be reserved here** or short codes can shadow them. Custom codes may contain CJK ideographs; `Validate` counts runes, not bytes; multi-segment entries reserve a whole subtree

### cli

- Subcommands dispatched by `Main(args)`; `reset uablock|ipblock|sessions` are in-process config changes, storage resets signal PID 1 so the container's restart policy starts gourl again (indirected via `restartGourlFn` for tests)

### logx

- `log_level` is a config field (default info): hot-applied via `SetLevel`; `LOG_FORMAT` stays env-only. `LOG_DIR` mirrors to a lumberjack-rotated file
- **`logRequests` is the every-request logger**: status-graded (>=500 error / >=400 warning / else debug), never UA, only JSON bodies mirrored — keep the SSE and `POST /api/v1/tokens` exclusions. Info = business events with `actor`; request access logging stays debug
- `Subscribe()` fans records to live SSE subscribers; `ReadHistory(limit, offset)` parses the mirrored file back leniently

## Testing conventions

- `api` tests use `newTestServer(t)`: miniredis + `:memory:` store + mock fetcher (`errNoFetch`) + shared `testSession` cookie attached by `do()`; requests without auth use `doWith(..., nil, "")`
- The `now` clock is injectable (`srv.now = ...`) — tests pin timestamps, so list-ordering tests rely on `ORDER BY ..., rowid` tie-breaking
- E2E server (`cmd/e2e`) reuses the same packages; keep its admin password `e2e-password` stable (Playwright specs hardcode it); its config path must be a **writable temp file** (an empty `CONFIG_PATH` breaks the settings-page write-back and makes PUT /config fail)
- `errors.Is(err, store.ErrNotFound)` / `ErrTaken` are the store's sentinels; constraint detection matches SQLite PRIMARYKEY/UNIQUE codes
