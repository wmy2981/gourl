# gourl — Project Conventions

## Overview

Lightweight self-hosted URL shortener: Go backend (stdlib `net/http`, no framework) + React 19/TS SPA admin (Vite + Tailwind 4) + SQLite (links and related records only) + Redis click buffer (30s batch flush) + embedded Swagger at `/docs/`. Business config in `config.yaml`, runtime/secrets in env vars.

## Commands

- Build: `go build ./cmd/gourl` — needs the gitignored go:embed files `internal/webui/dist` + `internal/webui/icon.svg`; run `powershell -File scripts/build-frontend.ps1` (or `./scripts/build-frontend.sh`) first
- Test: `go test ./...` (miniredis + `:memory:`, no real Redis); Vet: `go vet ./...`
- Frontend: `cd frontend && npm ci && npm run typecheck && npm run test`
- E2E: `cd frontend && npm run e2e` (auto-starts cmd/e2e: in-memory SQLite + miniredis; port `8099` locally, CI derives one from `GITHUB_RUN_ID`). **The e2e server serves the embedded frontend — rebuild it after frontend changes or tests run stale UI**
- Container CLI (`gourl help` lists all): no args start the server; `reset <target>`, `db export`, `status`, `health`, `config show`, `setup-code`, `log`, `webui on|off`, `reload`, `restart`, `version`. Sensitive ops confirm on a terminal or take `-y`; storage resets and `restart` stop the process so the container's restart policy brings it back. **`gourl reload`** re-reads config.yaml via SIGHUP after hand edits — some config flags (e.g. `sql_console_enabled`) are only togglable that way

## Git Workflow

- Conventional Commits, English, imperative mood: `type(scope): lowercase description`
- **One logical change = one commit**; each change must carry its tests, and nothing is committed until all tests pass
- Branch model: `main` (release) + `dev` (pre-release); feature branches merge via PR
- **dev is the active development branch**: CI + Docker build must be green on dev before merging to main
- Version: single source of truth is the root `VERSION` file, maintained manually; release pipeline validates forward-only progression, unchanged versions skip (never fail). Docker builds inject `VERSION (sha7)` (dev branch) or the plain version (main) via the `VERSION_STR` build arg; local builds fall back to the VERSION file
- CI/build workflows have **no concurrency block**: every push runs its own full pipeline and never cancels an older run

## Engineering Rules

### Data & state

- **Deletes are soft** (links and tokens): DELETE/batch/clear-expired only set a flag; every read path, redirect route and export excludes them. Codes are unique **only among non-deleted rows** → freed codes are reusable; token keys stay permanently taken. Deleted rows are not recoverable
- Click stats are **permanent history**: totals/trends sum daily clicks keyed by `(link_id, date)` and survive link deletion — never "clean up" click records when deleting links. A reused short code counts from zero under its fresh id
- Pre-edit snapshots go to the append-only `backups` table (`b-1, b-2, …`) on manual edits, renames and batch conflict=update; click-count flushes never back up
- **`store.GetLink` is cached (TTL 60s)**, click counts included — every write must invalidate the affected code (create/update/rename/meta/delete), `ApplyCounts` drops touched codes, `DeleteExpired` clears the whole cache. Never bypass the invalidation rules

### Backend behavior

- stdlib `net/http` only; `modernc.org/sqlite` (no CGO); gofmt + vet clean
- Title/description fetching is **async** (`internal/api/metaqueue.go`) — never reintroduce a synchronous fetch on the request path (the old 5s timeout made creates feel hung)
- Batch import is **lenient** but `click_count` is never imported, `deleted: true` items are skipped, and conflict = error|skip|update. Import accepts both the wrapped export dump and a bare array
- UA blocks are config-managed (`ua_blocks` in settings); `/api/v1/ua-blocks` stays for programmatic use. **IP bans (`ip_blocks`) are the outermost middleware** (every route incl. health) → 403 page naming the rule
- Reserved short-code prefixes live in `internal/shortcode` — **new system routes must be added there** or codes can shadow them
- All user-facing strings are bilingual zh/en; API error `code`s are stable English identifiers; the frontend maps them
- The three link exports share one filename shape and one row schema plus export metadata; the batch import accepts both the wrapped dump and the legacy bare array
- New API endpoints must update `internal/webui/openapi.yaml` too — CI lints the spec with Redocly, so structural errors fail the build

### Frontend & Android

- Follow the frontend-design skill's two-pass process for UI work; UI accent amber (never default blue-purple gradients); detailed conventions live in `frontend/CLAUDE.md`
- **Icon single source**: `assets/favicon.svg` only — copies under `frontend/src/assets/icon.svg` and `internal/webui/icon.svg` are gitignored generated files; change the root source and re-run the syncs
- Android app (Capacitor WebView, **token mode**): stored `{url, token}` + Bearer, no session cookie — login/setup pages never appear. Downloads go through the native bridge (`GourlBridge.saveToDownloads` in MainActivity) into system **Downloads/gourl/**. The log stream is fetch-parsed SSE (EventSource cannot send a Bearer header). Back intercepts: Escape to an open dialog first, then `exitApp()`

### Ops

- Deployment is a **single container** (image embeds Redis; the entrypoint starts it unless `REDIS_ADDR` points elsewhere). `docker-compose.yml` binds `./data` and `./config` — **directory mounts only** — and loads a sibling `.env`
- No local Docker builds/deployments — images are built and pushed to GHCR (`ghcr.io/wmy2981/gourl`) exclusively by GitHub Actions
- The entrypoint runs as root only to chown fresh bind mounts, then drops privileges via `su-exec` (keep it installed and the entrypoint root-owned)
