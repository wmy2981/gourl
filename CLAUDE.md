# gourl — Project Conventions

## Overview

Lightweight self-hosted URL shortener. Go backend (stdlib `net/http`, no framework) + React 19/TS SPA admin (Vite + Tailwind 4 + shadcn-style components). SQLite stores links and related records only; business config lives in `config.yaml`; runtime/secrets come from env vars. Redis buffers click counts (30s batch flush). Swagger UI is served at `/docs/` from the embedded binary.

The full design is in [DESIGN.md](DESIGN.md) (Chinese). **It is the single source of truth for architecture decisions — read it before implementing any feature.**

## Commands

- Build: `go build ./cmd/gourl` (requires `internal/webui/dist` — run `powershell -File scripts/build-frontend.ps1` first)
- Test: `go test ./...` (Go tests use miniredis, no real Redis needed)
- Vet: `go vet ./...`
- Frontend: `cd frontend && npm ci && npm run typecheck && npm run test`
- E2E: `cd frontend && npm run e2e` (auto-starts cmd/e2e server: in-memory SQLite + miniredis). Port: local `8099`, CI derives a unique one per run from `GITHUB_RUN_ID`. **The e2e server serves the embedded frontend — rebuild `scripts/build-frontend.ps1` after frontend changes before running e2e**

## Git Workflow

- Conventional Commits, English, imperative mood: `type(scope): lowercase description`
- **One logical change = one commit**; each change must carry its tests, and nothing is committed until all tests pass
- Branch model: `main` (release) + `dev` (pre-release); feature branches merge via PR
- **dev is the active development branch**: CI + Docker build must be green on dev before merging to main
- Version: single source of truth is the root `VERSION` file, maintained manually; release pipeline validates forward-only progression, unchanged versions skip (never fail) — see `.github/scripts/release_check.py`. Docker builds inject `VERSION (sha7)` (dev branch) or the plain version (main) via the `VERSION_STR` build arg — both the Go `version.Version` (health/log) and the frontend `__APP_VERSION__` (footer/login); local builds fall back to the VERSION file
- CI/build workflows have **no concurrency block**: every push runs its own full pipeline and never cancels an older run

## Engineering Rules

- Go: stdlib only for HTTP; `modernc.org/sqlite` (no CGO — multi-platform builds); gofmt + go vet clean
- Logging: `log/slog` via `internal/logx` — 4 levels (debug/info/warning/error), `LOG_LEVEL`/`LOG_FORMAT`/`LOG_DIR` env (LOG_DIR mirrors logs to a rotating file on the mounted volume, e.g. `/app/data/log`), English messages only, no i18n
- Click stats are **permanent history**: totals/trend sum the `daily_clicks` table and link deletion keeps them — never "clean up" click records when deleting links
- UA block patterns are **config-managed** (`config.yaml` `ua_blocks`, comma-separated in the settings form); the `/api/v1/ua-blocks` API remains for programmatic use
- All user-facing strings (API errors, UI copy) are bilingual zh/en; site info fields in `config.yaml` are single-language
- No local Docker builds or deployments — images are built and pushed to GHCR (`ghcr.io/wmy2981/gourl`) exclusively by GitHub Actions workflows
- Reserved short-code prefixes (`api`, `admin`, `docs`, …) live in `internal/shortcode`; new system routes must be added there
- Follow the frontend-design skill's two-pass process for UI work. UI accent is amber (never default blue-purple gradients); the **brand icon is amber `#f59e0b`** (user-specified, shares the accent color)
- **Icon single source**: the brand icon exists only at `assets/favicon.svg`. `scripts/build-frontend.ps1` and CI copy it to `frontend/src/assets/icon.svg` (vite import) and `internal/webui/icon.svg` (go:embed) — both are gitignored generated copies. Never edit the copies directly
- Deployment is a **single container**: the image embeds Redis (entrypoint starts it on 127.0.0.1:6379 unless `REDIS_ADDR` points elsewhere); `docker-compose.yml` binds `./data` and `./config` (directory mounts only — a file mount for a missing file becomes a directory) and loads a sibling `.env`. `GOURL_IMAGE` overrides the image tag (`:dev` for pre-releases; `:latest` exists only after a main release)
