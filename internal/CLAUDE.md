# gourl internal — Go Backend Conventions

Subdirectory instructions for the Go backend packages. Root-level conventions (commits, branch model, versioning) live in the repository-root `CLAUDE.md` and apply here too.

## Commands

- Build: `go build ./cmd/gourl` — **fails without `internal/webui/dist` and `internal/webui/icon.svg`** (go:embed). Both are gitignored generated files; run `powershell -File scripts/build-frontend.ps1` (repo root) first to regenerate them
- Test: `go test ./...` (no real Redis needed — miniredis; SQLite via `:memory:`)
- Vet: `go vet ./...` — must stay clean
- Run: `go run ./cmd/gourl` (env: `ADMIN_PASSWORD`, `SESSION_SECRET`, `REDIS_ADDR`, `DB_PATH`, `CONFIG_PATH`, `LOG_LEVEL`)

## Package map (what lives where)

- `config` — YAML business config; `Manager.Update` writes back atomically and hot-swaps (settings page uses it). Add new config fields with both `yaml` and `json` tags. **`Get()` normalizes slice fields to empty (not nil) slices — JSON must never emit `null` arrays (the frontend crashed on `null.join()` once)**
- `store` — SQLite (modernc.org/sqlite, **no CGO**). Schema migrations are the ordered `migrations` slice; add new tables there, never raw DDL
- `counter` — Redis click buffer + 30s `Flusher` (GETDEL → ApplyCounts → INCRBY add-back on failure). Keys: `counter:{code}` / `counter:{code}:{date}`
- `shortcode` — base62 random codes, multi-level validation, **`builtinReserved` prefixes** (`api`, `admin`, `docs`, …). New system routes MUST be added here or short codes can shadow them
- `api` — stdlib `net/http` mux (Go 1.22+ patterns, `{code...}` wildcards). Multi-level codes come from `PathValue` with possible leading slash — always route through `pathCode()`
- `fetcher` — title/description scraping with SSRF checks (private ranges, per-hop validation, 5s timeout, 1 MiB cap)
- `webui` — go:embed of frontend dist + swagger-ui + `openapi.yaml` + icon. **Adding a new API endpoint means updating `openapi.yaml` too**
- `logx` — slog setup, 4 levels, `LOG_LEVEL`/`LOG_FORMAT` env. Log in English; use `slog.Warn`/`Error` for failures, never plain `log.Printf`

## Testing conventions

- `api` tests use `newTestServer(t)`: miniredis + `:memory:` store + mock fetcher (`errNoFetch`) + shared `testSession` cookie attached by `do()`. Requests without auth use `doWith(..., nil, "")`
- The `now` clock is injectable (`srv.now = ...`) — tests pin timestamps, so list ordering tests rely on `ORDER BY ... , rowid` tie-breaking
- Redis keys are asserted through the miniredis handle returned by `newTestServer`
- E2E server (`cmd/e2e`) reuses the same packages with miniredis + in-memory SQLite; keep its admin password `e2e-password` stable (Playwright specs hardcode it). Its config path must be a **writable temp file** (an empty `CONFIG_PATH` breaks the settings-page write-back and makes PUT /config fail)

## Gotchas

- `errors.Is(err, store.ErrNotFound)` / `ErrTaken` are the store's sentinels; constraint detection matches SQLite PRIMARYKEY/UNIQUE codes
- `:memory:` SQLite is per-connection — `store.Open(":memory:")` in tests is fine, but a shared in-memory DB across goroutines needs a file DSN
- Frontend embed path is package-relative: `internal/webui/dist` must exist for every `go test` — CI builds it before running tests
