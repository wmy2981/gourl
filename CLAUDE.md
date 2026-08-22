# gourl — Project Conventions

## Overview

Lightweight self-hosted URL shortener: Go backend (stdlib `net/http`, no framework) + React 19/TS SPA admin (Vite + Tailwind 4) + SQLite (links and related records only) + Redis click buffer (30s batch flush) + embedded Swagger at `/docs/`. Business config in `config.yaml`, runtime/secrets in env vars.

Beyond the basics: batch create/delete/import-export, live SSE log page, QR codes with JPEG download, multi base URLs, CJK + multi-level short codes, async title fetching, per-link descriptions, edit snapshots (`b-1, b-2, …`), full soft deletion, custom themed controls, IP bans + per-IP login lockout + per-second redirect budget, setup-flow admin password + bootstrap code, session TTL with epoch revocation, container CLI, Capacitor Android app (token mode, `gourl://` deep links).

## Commands

- Build: `go build ./cmd/gourl` — needs the gitignored go:embed files `internal/webui/dist` + `internal/webui/icon.svg`; run `powershell -File scripts/build-frontend.ps1` (or `./scripts/build-frontend.sh`) first
- Test: `go test ./...` (miniredis + `:memory:`, no real Redis); Vet: `go vet ./...`
- Frontend: `cd frontend && npm ci && npm run typecheck && npm run test`
- E2E: `cd frontend && npm run e2e` (auto-starts cmd/e2e: in-memory SQLite + miniredis; port `8099` locally, CI derives one from `GITHUB_RUN_ID`). **The e2e server serves the embedded frontend — rebuild it after frontend changes or tests run stale UI**

## Git Workflow

- Conventional Commits, English, imperative mood: `type(scope): lowercase description`
- **One logical change = one commit**; each change must carry its tests, and nothing is committed until all tests pass
- Branch model: `main` (release) + `dev` (pre-release); feature branches merge via PR
- **dev is the active development branch**: CI + Docker build must be green on dev before merging to main
- Version: single source of truth is the root `VERSION` file, maintained manually; release pipeline validates forward-only progression, unchanged versions skip (never fail) — see `.github/scripts/release_check.py`. Docker builds inject `VERSION (sha7)` (dev branch) or the plain version (main) via the `VERSION_STR` build arg — both the Go `version.Version` (health/log) and the frontend `__APP_VERSION__` (footer/login); local builds fall back to the VERSION file
- CI/build workflows have **no concurrency block**: every push runs its own full pipeline and never cancels an older run

## Engineering Rules

### Data & state

- **Deletes are soft** (links and tokens): DELETE/batch/clear-expired only set a flag; every read path, redirect route and export excludes them. Codes are unique **only among non-deleted rows** (partial index) → freed codes are reusable; token keys stay permanently taken. Deleted rows are not recoverable
- Click stats are **permanent history**: totals/trends sum `daily_clicks` (keyed by `(link_id, date)` since migration v5) and survive link deletion — never "clean up" click records when deleting links (batch delete and clear-expired follow the same rule). A reused short code counts from **zero** under its fresh id (`ApplyCounts` resolves the live id per flush)
- Pre-edit snapshots go to the append-only `backups` table (`b-1, b-2, …`) on manual edits, renames and batch conflict=update; click-count flushes never back up
- **`store.GetLink` is cached (TTL 60s)**, click counts included — every write must invalidate the affected code (create/update/rename/meta/delete), `ApplyCounts` drops touched codes, `DeleteExpired` clears the whole cache. Never bypass the invalidation rules

### Backend behavior

- stdlib `net/http` only; `modernc.org/sqlite` (no CGO — multi-platform builds); gofmt + vet clean
- Logging (`internal/logx`, 4 levels): `log_level` is a **business config field** (settings page, hot-applied via `logx.SetLevel`); `LOG_DIR` mirrors to a rotating file. **Every request is logged status-graded** (>=500 error / >=400 warning / else debug — invalid/refused requests must be visible as warnings), no UA, only JSON response bodies mirrored (flattened attrs; keep the SSE and `POST /api/v1/tokens` exclusions). Info = business events with `actor` (session|token|app — app requests are identified by their `gourl/<version>` UA and additionally logged with `app_version` + `token_id`); link-operation logs carry `code` + `id` (batch operations: counts + `first_code`/`first_id`); debug = handler details + every store write (`store:` prefix). `logx` fans records to the log-page SSE and reads history back from the LOG_DIR file
- **Admin password** is a bcrypt hash in `config.yaml` (hidden from the JSON contract). No hash → setup mode: management endpoints refuse with 403 `setup_required` (bearer tokens still work; `/admin/setup`, `/api/v1/auth/status`, `/api/v1/health`, redirects stay open). Setup also requires the **bootstrap code** (8-char, printed once per process — dies on restart). `POST /api/v1/auth/change-password` **bumps `session_epoch` → revokes every session, the changer's included** (bearer tokens unaffected). `PUT /api/v1/config` must carry over every `json:"-"` field (`password_hash`, `session_epoch`, `webui_enabled`) or it gets wiped
- **Sessions are stateless `exp.epoch.nonce.hmac` cookies**; TTL applies at issue time only; the `epoch` claim must match `config.session_epoch`. `SESSION_SECRET` unset → ephemeral per-process secret (sessions do not survive a restart). `adminAuth` re-resolves per request, so hash/epoch changes apply without a restart
- **API tokens are bcrypt-hashed at rest** (migration v6): the plaintext appears only in the create response and as an 8-char `token_prefix` column (UI preview); `GetToken` compares every active hash (counts are tiny) and `CreateToken` refuses a reused key with `ErrTaken` — keys stay permanently taken, soft-deleted rows included. `MigrateTokenHashes` rewrites legacy plaintext rows in place at startup; both db exporters carry `token_prefix`
- Title/description fetching is **async** (`internal/api/metaqueue.go`) — never reintroduce a synchronous fetch on the request path (the old 5s timeout made creates feel hung). **Any reachable host is allowed** (no SSRF filtering — only an authenticated admin triggers fetches); the 5s timeout, 5-hop redirect cap and 1 MiB body limit stay; non-http(s) targets never enter the queue
- Batch import is **lenient** (case-insensitive fields, coercion, nulls defaulted; only a missing url fails) — but `click_count` is never imported, `deleted: true` items are skipped, and codes held by soft-deleted rows are free. Conflict = error|skip|update
- UA blocks are **config-managed** (`ua_blocks`, comma-separated in settings); `/api/v1/ua-blocks` stays for programmatic use. **IP bans (`ip_blocks`) are the outermost middleware** (every route incl. health): exact IP / CIDR / `192.168.*.*` wildcards → 403 page naming the rule (shared `renderBlocked`)
- All user-facing strings are bilingual zh/en; site info fields in `config.yaml` are single-language. API error `code`s are stable English identifiers; the frontend maps them
- Reserved short-code prefixes live in `internal/shortcode` — **new system routes must be added there**. Custom codes may contain simplified Chinese; `MaxLength` counts runes, not bytes; multi-segment entries reserve a whole subtree
- Export downloads share one filename shape — `gourl-links|logs-<local yyyy-mm-dd-hh-mm-ss>.{csv,json,md,log}`: the frontend names them via `exportFilename` (download.ts, local time), the backend CSV/markdown `Content-Disposition` matches (server-local time)
- The three link exports (`/api/v1/links/export.csv|json|md`) carry the same 7 fields plus **export metadata**: JSON wraps rows as `{meta: {site, version, count, exported_at}, items: [...]}`, CSV prepends `#`-comment metadata lines, markdown uses YAML front matter (English body, `code`/`url`/... column names). **The batch import accepts both** the wrapped dump and the legacy bare array (`decodeBatchRequest` ignores unknown top-level keys; ImportDialog unwraps `.items`; CSV import skips `#` lines)

### Frontend & Android

- Follow the frontend-design skill's two-pass process for UI work; UI accent amber (never default blue-purple gradients); detailed conventions live in `frontend/CLAUDE.md`
- **Icon single source**: `assets/favicon.svg` only — `frontend/src/assets/icon.svg` and `internal/webui/icon.svg` are gitignored generated copies; never edit copies, change the root source and re-run the syncs
- Android app (Capacitor WebView, **token mode**): stored `{url, token}` + Bearer, no session cookie — login/setup pages never appear; the connect screen probes the server before persisting. Downloads go through `@capgo/capacitor-file-sharer` into system **Downloads/gourl/** — never the Filesystem plugin (its v8 enum lost `Downloads`). The log stream is fetch-parsed SSE (EventSource cannot send a Bearer header). **Back intercepts**: Escape to an open dialog first, then `exitApp()` (the Android 13+ predictive-back animation is traded for this control on purpose). The settings connection card probes `GET /api/v1/config` every 10s + on foreground — that endpoint must stay behind `requireAuth`. **No adaptive-icon resources**: legacy mipmaps let the platform scale/mask the glyph itself — hand-written VectorDrawable transforms (translate-then-scale) landed it off-center or invisible

### Ops

- **Container CLI** (`internal/cli`, dispatched by `cmd/gourl`; no args start the server): `reset password|uablock|ipblock|config|sessions|api|db|redis|--all`, `db export [out-dir]`, `status`, `health`, `config show` (password hash hidden), `setup-code`, `log [lines]`, `webui on|off`, `restart`, `version`. Sensitive ops confirm on a terminal or take `-y`; storage resets and `restart` stop the process (SIGINT to PID 1) so the container's restart policy starts it again. `webui_enabled` gates `/admin` only — Swagger `/docs` stays up
- Deployment is a **single container** (image embeds Redis; the entrypoint starts it unless `REDIS_ADDR` points elsewhere). `docker-compose.yml` binds `./data` and `./config` — **directory mounts only** (a file mount for a missing file becomes a directory) — and loads a sibling `.env`; `GOURL_IMAGE` overrides the image tag (`:dev` for pre-releases)
- No local Docker builds/deployments — images are built and pushed to GHCR (`ghcr.io/wmy2981/gourl`) exclusively by GitHub Actions
- First deployment needs no manual chmod: the entrypoint runs as root only to chown fresh `./data`/`./config` bind mounts, then drops privileges via `su-exec` (keep `su-exec` installed and the entrypoint root-owned)
