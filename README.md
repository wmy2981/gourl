# gourl

> [中文文档](README.zh-CN.md) | English

<p align="left">
  <img src="assets/favicon.svg" width="72" height="72" alt="gourl" />
</p>

Lightweight self-hosted URL shortener. A single Go binary (frontend embedded)
plus Redis — that's all it takes to run your own short links.

## Features

- **Short links** — custom or auto-generated codes, multi-level paths (`link1/link2`), configurable length
- **Link management** — modern admin console (Apple-style glassmorphism, responsive, dark mode)
- **Delayed counting** — clicks buffered in Redis, flushed to SQLite every 30s to absorb bursts
- **History preserved** — click totals and the trend chart keep counting even after a link is deleted
- **Auto titles** — fetches `title`/`description` when creating a link, with SSRF protection
- **UA blocking** — User-Agent patterns (comma-separated in Settings) get 403 and are never counted
- **Expiry** — per-link `expires_at` (0 = never), graceful bilingual expired page
- **REST API** — full JSON API with bearer tokens for integration
- **Customization** — site name, title, keywords, header/footer, uploaded icon (SVG/PNG, one-click reset)
- **Multi-base URLs** — extra base URLs are served side by side; pick which one a link row shows or copies
- **i18n** — English and Chinese, auto-detected with a manual switcher
- **QR codes, CSV/JSON export, batch import** — paste JSON or load a `.csv`/`.json` file
- **API documentation** — interactive Swagger UI at `/docs/`
- **Structured logging** — slog with 4 levels (debug/info/warning/error), text or JSON, optionally mirrored to a rotating file on the data volume

## Stack

Go (stdlib `net/http`, `log/slog`) · SQLite ([modernc](https://modernc.org/sqlite), no CGO) ·
Redis · React 19 + Vite + Tailwind CSS 4 + shadcn-style components

## Quick start (Docker)

Single container — the image embeds a Redis instance, so one `compose up`
is the whole deployment.

```bash
# 1. Config directory (optional — defaults work without it):
#    copy config.yaml.example to config/config.yaml and adjust as needed.

# 2. Create the .env file next to docker-compose.yml:
echo 'ADMIN_PASSWORD=change-me' > .env
echo "SESSION_SECRET=$(openssl rand -hex 32)" >> .env

# 3. Start
docker compose up -d
```

Open http://localhost:8080 — you'll be redirected to the admin console.
Data (SQLite, uploaded icons, embedded Redis rdb, rotating logs) persists in
`./data`, config in `./config` (the settings page writes back to it).

Images are built and published by GitHub Actions on [GHCR](https://github.com/wmy2981/gourl/pkgs/container/gourl):
`ghcr.io/wmy2981/gourl:latest` (releases, main branch) or `:dev` (pre-releases).
To deploy a pre-release: `GOURL_IMAGE=ghcr.io/wmy2981/gourl:dev docker compose up -d`.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `ADMIN_PASSWORD` | — (auth disabled) | Admin console password; empty = trusted-network mode |
| `SESSION_SECRET` | insecure default | Signs admin session cookies — **set it in production** |
| `REDIS_ADDR` | `localhost:6379` | Click-counter buffer |
| `DB_PATH` | `data/gourl.db` | SQLite file |
| `PORT` | `8080` | HTTP listen port |
| `CONFIG_PATH` | `config.yaml` | Business config (site info, base URLs, …) |
| `ASSETS_DIR` | `data/assets` | Uploaded icon storage |
| `TZ` | container default | Daily click buckets and expiry are interpreted in it |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warning` / `error` |
| `LOG_FORMAT` | `text` | `json` for structured output (logs go to stderr) |
| `LOG_DIR` | — | Optional directory for a rotating file mirror of the logs (e.g. `/app/data/log` on the mounted volume; 10 MB × 5 backups × 30 days, gzip) |

Business settings live in `config.yaml` (see `config.yaml.example`): site
name/title/keywords/description/header/footer, random code length, primary +
extra base URLs, extra reserved codes, User-Agent block patterns, and the
custom icon.

## API documentation

Open http://localhost:8080/docs/ — an interactive Swagger UI covering every
endpoint, served from the single binary (`/docs/openapi.yaml` is the raw
OpenAPI 3.0 spec).

## API

Base path `/api/v1`. Admin endpoints accept a session cookie or
`Authorization: Bearer <token>` (tokens are created in Settings).

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/health` | **Public**: name, version, uptime, redis/sqlite probes |
| POST | `/api/v1/auth/login` | Password login → session cookie |
| GET/POST | `/api/v1/links` | List (paged, searchable) / create |
| POST | `/api/v1/links/batch` | Batch import (≤500 per call) |
| GET/PATCH/DELETE | `/api/v1/links/{code}` | Detail / update / delete |
| GET | `/api/v1/links/{code}/stats` | Total + daily click counts |
| GET | `/api/v1/export.csv` | Export all links as CSV |
| GET | `/api/v1/export.json` | Export all links as JSON |
| GET/POST/DELETE | `/api/v1/ua-blocks` | Blocked User-Agent patterns (programmatic use; the settings page manages them via config) |
| GET/POST/DELETE | `/api/v1/tokens` | API tokens |
| GET/PUT | `/api/v1/config` | Site config (hot-applied, written back to YAML) |
| POST/DELETE | `/api/v1/icon` | Custom icon upload / reset |
| GET | `/api/v1/dashboard` | Aggregate metrics + 14-day trend |

Redirect: `GET /{code}` → 302 to the target (reserved prefixes like
`api`, `admin`, `expired` never collide with short codes).

## Development

```bash
# Backend tests (miniredis, no real Redis needed)
go test ./...

# Frontend (vitest + type check)
cd frontend && npm ci && npm run test && npm run typecheck

# End-to-end (starts its own server: in-memory SQLite + miniredis)
cd frontend && npm run e2e

# Build the full binary (builds frontend, embeds it)
powershell -File scripts/build-frontend.ps1 && go build ./cmd/gourl
```

Conventions: Conventional Commits (one logical change = one commit, tests
included), `main` releases / `dev` pre-releases, version maintained manually
in the root `VERSION` file — CI validates forward-only progression and
generates release notes automatically.

## License

[MIT](LICENSE)
