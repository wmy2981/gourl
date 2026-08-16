# gourl internal — Go Backend Conventions

Subdirectory instructions for the Go backend packages. Root-level conventions (commits, branch model, versioning) live in the repository-root `CLAUDE.md` and apply here too.

## Commands

- Build: `go build ./cmd/gourl` — **fails without `internal/webui/dist` and `internal/webui/icon.svg`** (go:embed). Both are gitignored generated files; run `powershell -File scripts/build-frontend.ps1` (repo root) first to regenerate them
- Test: `go test ./...` (no real Redis needed — miniredis; SQLite via `:memory:`)
- Vet: `go vet ./...` — must stay clean
- Run: `go run ./cmd/gourl` (env: `ADMIN_PASSWORD`, `SESSION_SECRET`, `REDIS_ADDR`, `DB_PATH`, `CONFIG_PATH`, `LOG_LEVEL`)

## Package map (what lives where)

- `config` — YAML business config; `Manager.Update` writes back atomically and hot-swaps (settings page uses it). Add new config fields with both `yaml` and `json` tags. **`Get()` normalizes slice fields to empty (not nil) slices — JSON must never emit `null` arrays (the frontend crashed on `null.join()` once)**
- `store` — SQLite (modernc.org/sqlite, **no CGO**). Schema migrations are the ordered `migrations` slice; add new tables there, never raw DDL. **`daily_clicks` is permanent history**: `DeleteLink` must not remove it (dashboard totals/trend sum the daily table, so clicks survive link deletion)
- `counter` — Redis click buffer + 30s `Flusher` (GETDEL → ApplyCounts → INCRBY add-back on failure). Keys: `counter:{code}` / `counter:{code}:{date}`
- `shortcode` — base62 random codes, multi-level validation, **`builtinReserved` prefixes** (`api`, `admin`, `docs`, …). New system routes MUST be added here or short codes can shadow them. Custom codes may contain CJK ideographs (simplified Chinese); `Validate` counts runes, not bytes
- `api` — stdlib `net/http` mux (Go 1.22+ patterns, `{code...}` wildcards). Multi-level codes come from `PathValue` with possible leading slash — always route through `pathCode()`. **`ua_blocks` is a regular config field** — the settings page PUTs it wholesale; `/api/v1/ua-blocks` stays for programmatic use (updating one field still means `cfg.Get()` → mutate → `cfg.Update`). Beyond the CRUD surface there are: `DELETE /api/v1/links` (batch delete by codes), `GET|DELETE /api/v1/links/expired` (count / clear), `GET /api/v1/logs` (history from the LOG_DIR file) and `GET /api/v1/logs/stream` (SSE). `requireAuth` stamps the request context with `actor` (session|token) — business-event logs must carry `actorFrom(r)`
- `fetcher` — title/description scraping with SSRF checks (private ranges, per-hop validation, 5s timeout, 1 MiB cap). **Never call it synchronously in a handler**: the `metaQueue` in `api/metaqueue.go` fetches in the background and persists via `store.UpdateMeta`
- `webui` — go:embed of frontend dist + swagger-ui + `openapi.yaml` + icon. **Adding a new API endpoint means updating `openapi.yaml` too**
- `logx` — slog setup, 4 levels, `LOG_LEVEL`/`LOG_FORMAT`/`LOG_DIR` env. `LOG_DIR` mirrors logs to a lumberjack-rotated file (10MB × 5 × 30d, gzip) on the mounted volume; stderr stays in the chain. `Close()` releases the file handle. Log in English; use `slog.Warn`/`Error` for failures, never plain `log.Printf`. `Subscribe()` fans every record to live subscribers (the SSE log stream) and `ReadHistory(limit, offset)` parses the mirrored file back (text and JSON lines, leniently). **Request access logging is at debug** (`logRequests` in api.go) — info is reserved for business events with `actor`

## Testing conventions

- `api` tests use `newTestServer(t)`: miniredis + `:memory:` store + mock fetcher (`errNoFetch`) + shared `testSession` cookie attached by `do()`. Requests without auth use `doWith(..., nil, "")`
- The `now` clock is injectable (`srv.now = ...`) — tests pin timestamps, so list ordering tests rely on `ORDER BY ... , rowid` tie-breaking
- Redis keys are asserted through the miniredis handle returned by `newTestServer`
- E2E server (`cmd/e2e`) reuses the same packages with miniredis + in-memory SQLite; keep its admin password `e2e-password` stable (Playwright specs hardcode it). Its config path must be a **writable temp file** (an empty `CONFIG_PATH` breaks the settings-page write-back and makes PUT /config fail)

## Gotchas

- `errors.Is(err, store.ErrNotFound)` / `ErrTaken` are the store's sentinels; constraint detection matches SQLite PRIMARYKEY/UNIQUE codes
- `:memory:` SQLite is per-connection — `store.Open(":memory:")` in tests is fine, but a shared in-memory DB across goroutines needs a file DSN
- Frontend embed path is package-relative: `internal/webui/dist` must exist for every `go test` — CI builds it before running tests
