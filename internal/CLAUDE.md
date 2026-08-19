# gourl internal — Go Backend Conventions

Subdirectory conventions for the Go backend packages. Root-level conventions (commits, branch model, versioning) live in the repository-root `CLAUDE.md` and apply here too.

## Commands

- Build/run: `go build ./cmd/gourl` / `go run ./cmd/gourl` — **build fails without the gitignored go:embed files** `internal/webui/dist` + `internal/webui/icon.svg`; run `powershell -File scripts/build-frontend.ps1` (repo root) first
- Test: `go test ./...` (miniredis + `:memory:`, no real Redis; CI builds the frontend embed before testing)
- Vet: `go vet ./...` — must stay clean
- Defaults: `./config/config.yaml`, `./data/gourl.db`, `./data/log`; env overrides live in cmd/gourl/main.go

## Principles

- stdlib `net/http` only (Go 1.22+ patterns, `{code...}` wildcards); `modernc.org/sqlite` (no CGO — multi-platform builds); gofmt + vet clean
- Log in English via `slog` only, never `log.Printf`; all user-facing strings bilingual zh/en — API error `code`s are stable English identifiers the frontend maps
- SQLite stores links and related records only; business config in `config.yaml`; runtime/secrets in env vars

## Package conventions

### config

- `Manager.Update` writes back atomically and hot-swaps (the settings page relies on it)
- **`Get()` normalizes slice fields to empty (not nil) slices — JSON must never emit `null` arrays** (the frontend crashed on `null.join()` once)
- Three fields are `json:"-"`: `password_hash`, `session_epoch`, `webui_enabled` — never expose them, and `updateConfig` must carry all three over from the live config or a PUT wipes/resets them

### store

- Schema changes go through the ordered `migrations` slice (currently v1–v5, details in code comments) — never raw DDL
- **Soft deletes** (links + tokens): only `deleted = 1`; the partial index `idx_links_code_active` keeps codes unique among non-deleted rows (freed codes reusable, token keys permanently taken); every read path, the redirect route and the API exports exclude deleted rows; only db-export surfaces them
- **API tokens are bcrypt hashes** (v6, `token_prefix` keeps the UI preview): hashes cannot match by SQL equality, so `GetToken` scans active rows (counts are tiny) and `CreateToken`'s plaintext check enforces "keys stay permanently taken" (`ErrTaken`) — writes close their read result sets first (the `:memory:` test store pins one connection)
- **`daily_clicks` is permanent history keyed by `(link_id, date)`** (v5): pre-v5 orphans keep a NULL `link_id` and still feed totals; `ApplyCounts` resolves the live id per flush so a reused code counts from zero — never clean up click rows on link deletion
- `backups` is append-only: `BackupLink` snapshots the pre-edit row with a 1-based `b_id` (exported as `b-1, b-2, …`); the api layer backs up on manual edits, renames and batch conflict=update, never on count flushes
- `DeleteLinks`/`DeleteExpired` return the first link actually deleted (request order / earliest expiry) so api logs can name a concrete `code` + `id`
- **`GetLink` serves from an in-memory TTL cache (60s)**, totals included — every write path must invalidate: create/update/rename/meta/delete del the code, `ApplyCounts` dels touched codes, `DeleteExpired` clears the whole cache
- `:memory:` SQLite is per-connection — fine for tests; a shared in-memory DB across goroutines needs a file DSN

### api

- Target URLs accept **any scheme** (`isAbsoluteURL`: non-empty scheme + content after it — `tcp://`, `openapp://`, `mailto:` pass, scheme-less inputs fail); only `base_url`/`extra_base_urls` stay http(s); non-http(s) targets skip title fetching
- `selfLinkTarget` rejects http(s) targets whose host:port matches a base URL (or the request host when unset) with a non-reserved first path segment (`self_link_target`); http vs https never match (effective ports 80/443). Descriptions are capped at 500 runes (`description_too_long`)
- Multi-level codes come from `PathValue` with a possible leading slash — always route through `pathCode()`
- `site.description`/`site.keywords` are injected into **every** served page: the SPA shell at serve time (`spaIndex`/`seoMeta`) and the 404/403 templates (empty values skipped)
- **`ip_blocks` is the outermost middleware** (every route incl. health): exact IP / CIDR / dotted-quad wildcards → 403 via `renderBlocked(w, r, "ip", rule)` (UA blocks share it with `"ua"`). The `cors` middleware sits inside `ipBlock`, so bans still cover preflights
- CORS: `Access-Control-Allow-Origin: *` on every `/api/` response — safe only because the app authenticates with Bearer tokens, no cookies; non-API routes carry no CORS headers
- Login rate limit (`loginLimiter`, keyed by `clientIP(r)`) is checked before password work; the link redirect budget is a shared `rate.Limiter` rebuilt when `link_rate_per_second` changes
- Auth resolves per request via `Server.adminAuth()` (lazy-syncs on `password_hash` change → password changes apply without a restart): config hash wins, legacy `ADMIN_PASSWORD` migrates once into the config file, neither → setup mode (403 `setup_required`; bearer tokens still work; `/admin/setup`, `/api/v1/auth/status`, `/api/v1/health` and short-link redirects stay open)
- Setup requires the **bootstrap code**: `NewServer` generates an 8-char mixed-alphanumeric code, prints it to terminal + log, persists it as `setup.code` next to the database (0600 — `gourl setup-code` reads it), constant-time-compares it (`invalid_setup_code`, no rate limit), then removes the file
- `POST /api/v1/auth/change-password` (login limiter applies) writes a fresh hash and **bumps `session_epoch`** — every session, the changer's included, is revoked; bearer tokens unaffected
- Sessions are stateless `exp.epoch.nonce.hmac` (4 parts; exp 0 = never expires): TTL applies at issue time only; `verifyToken` checks the epoch against `config.session_epoch` (bumping it revokes everything); `SESSION_SECRET` unset → ephemeral per-process secret (sessions do not survive a restart)
- `requireAuth` stamps the request context with `actor` (session|token|app) — bearer requests whose UA starts with `gourl/<version>` (the Capacitor WebView) become `actor=app` and gain `app_version` + `token_id`; business-event logs must go through `logInfo`/`logWarn` (they append `actorAttrs(r)`)
- **`GET /api/v1/config` doubles as the app's connection probe** (polled every 10s + on app foreground: 200 = connected, 401 = dead token, network error = unreachable) — keep it behind `requireAuth`, never public, or the probe silently reports the wrong state
- New API endpoints must update `internal/webui/openapi.yaml` too

### fetcher

- **Any reachable host is allowed** (no SSRF filtering — fetches are only triggered by an authenticated admin); every hop must still be an absolute http(s) URL, with a 5s timeout, 5-hop redirect cap and 1 MiB body limit
- Lenient by design: non-200 statuses and non-html content types are parsed anyway (internal services answer oddly yet carry a `<title>`); a fetch finding nothing must **not** wipe existing meta — the metaQueue worker skips `UpdateMeta` when title and description are both empty; failures log at warning level

### shortcode

- `builtinReserved` prefixes (`api`, `admin`, `docs`, …) — **new system routes MUST be reserved here** or short codes can shadow them. Custom codes may contain CJK ideographs; `Validate` counts runes, not bytes. Extra reserved entries may be Chinese or multi-segment: single-segment matches the first segment, multi-segment reserves the whole subtree

### cli

- Subcommands (`Main(args) int`; no args start the server): `reset password|uablock|ipblock|config|sessions|api|db|redis|--all`, `db export [out-dir]` (Go port of `scripts/db-export.mts` — keep the two outputs in sync), `status`, `health`, `config show`, `setup-code`, `log`, `webui`, `restart`, `version`
- Sensitive ops use `confirm(yes, prompt)` (terminal prompt or `-y`); storage resets (`reset config|db|redis|--all`) and `restart` signal PID 1 via `restartGourlFn` (indirected for tests) so the container's restart policy starts gourl again; `reset uablock|ipblock|sessions` are in-process config changes without a restart

### logx

- `log_level` is a config field (default info): `Init(level)` at startup, `SetLevel` at runtime (settings save calls it), `ParseLevel` maps config strings; `LOG_FORMAT` stays env-only. `LOG_DIR` (default `./data/log`, the mounted `/app/data/log` in the container) mirrors to a lumberjack-rotated file (10MB × 5 × 30d, gzip); `Close()` releases the handle
- **`logRequests` is the every-request logger**: status-graded (>=500 error / >=400 warning / else debug), never UA, only JSON bodies mirrored — flattened into key-value attrs by `parseJSONAttrs`; keep the SSE and `POST /api/v1/tokens` exclusions. Request access logging stays debug — info is reserved for business events with `actor`
- `Subscribe()` fans every record to live subscribers (the SSE log stream); `ReadHistory(limit, offset)` parses the mirrored file back (text and JSON lines, leniently)

## Testing conventions

- `api` tests use `newTestServer(t)`: miniredis + `:memory:` store + mock fetcher (`errNoFetch`) + shared `testSession` cookie attached by `do()`; requests without auth use `doWith(..., nil, "")`
- The `now` clock is injectable (`srv.now = ...`) — tests pin timestamps, so list-ordering tests rely on `ORDER BY ..., rowid` tie-breaking
- Redis keys are asserted through the miniredis handle returned by `newTestServer`
- E2E server (`cmd/e2e`) reuses the same packages; keep its admin password `e2e-password` stable (Playwright specs hardcode it); its config path must be a **writable temp file** (an empty `CONFIG_PATH` breaks the settings-page write-back and makes PUT /config fail)
- `errors.Is(err, store.ErrNotFound)` / `ErrTaken` are the store's sentinels; constraint detection matches SQLite PRIMARYKEY/UNIQUE codes
