# gourl

> [中文文档](README.zh-CN.md) | English

Lightweight self-hosted URL shortener. A single Go binary (frontend embedded)
plus Redis — that's all it takes to run your own short links.

## Features

- **Short links** — custom or auto-generated codes, multi-level paths (`link1/link2`), configurable length
- **Link management** — modern admin console (Apple-style glassmorphism, responsive, dark mode)
- **Delayed counting** — clicks buffered in Redis, flushed to SQLite every 30s to absorb bursts
- **Auto titles** — fetches `title`/`description` when creating a link, with SSRF protection
- **UA blocking** — admin-defined User-Agent patterns get 403 and are never counted
- **Expiry** — per-link `expires_at` (0 = never), graceful bilingual expired page
- **REST API** — full JSON API with bearer tokens for integration
- **Customization** — site name, title, keywords, header/footer, uploaded icon (SVG/PNG)
- **i18n** — English and Chinese, auto-detected with a manual switcher
- **QR codes, CSV export, batch import**

## Stack

Go (stdlib `net/http`) · SQLite ([modernc](https://modernc.org/sqlite), no CGO) ·
Redis · React 19 + Vite + Tailwind CSS 4 + shadcn-style components

## Quick start (Docker)

```bash
# 1. Create your config (optional — defaults work out of the box)
cp config.yaml.example config.yaml

# 2. Set the admin password and start
ADMIN_PASSWORD=change-me docker compose up -d
```

Open http://localhost:8080 — you'll be redirected to the admin console.
Images are built and published on GitHub Actions; pull tags like
`ghcr.io/wmy2981/gourl:latest` (releases) or `:dev` (pre-releases) from
[GHCR](https://github.com/wmy2981/gourl/pkgs/container/gourl).

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

Business settings live in `config.yaml` (see `config.yaml.example`): site
name/title/keywords/description/header/footer, random code length, primary +
extra base URLs, extra reserved codes, and the custom icon.

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
| GET | `/api/v1/export.csv` | Export all links |
| GET/POST/DELETE | `/api/v1/ua-blocks` | Blocked User-Agent patterns |
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
