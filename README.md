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
- **Delayed counting** — clicks buffered in Redis, flushed to SQLite every 30s to absorb bursts; lookups are served from an in-memory TTL cache
- **History preserved** — click totals and the trend chart keep counting even after a link is deleted
- **Auto titles** — fetches `title`/`description` when creating a link, with SSRF protection
- **UA & IP blocking** — User-Agent patterns and IP rules (exact IP, CIDR, `192.168.*.*` wildcards) get a 403 naming the matched rule and are never counted; IP bans cover every route
- **Expiry** — per-link `expires_at` (0 = never); expired codes behave like missing ones (plain 404)
- **REST API** — full JSON API with bearer tokens for integration
- **Setup flow** — on first start with no password the first visitor sets one via `/admin/setup`; the bcrypt hash lives in `config.yaml`, never in the environment
- **Rate limiting** — per-IP login lockout (default 10 failures / 300 s) and a shared per-second redirect budget (default 100/s), both configurable
- **Customization** — site name, title, keywords, description, uploaded icon (SVG/PNG, one-click reset)
- **Multi-base URLs** — extra base URLs are served side by side; pick which one a link row shows or copies
- **i18n** — English and Chinese, auto-detected with a manual switcher
- **QR codes, CSV/JSON export, batch import** — paste JSON or load a `.csv`/`.json` file; import conflicts resolve as error/skip/update with per-status counts; parsing is lenient (case-insensitive fields, date formats, number/string coercion); `click_count` is never imported
- **Batch ops** — bulk create (one strict-syntax line per link), cross-page bulk delete, one-click clear-expired, expiry filter with expired-row highlighting
- **Live log page** — Server-Sent Events stream with level/keyword/time filters and `.log` export; history from the mirrored file
- **Chinese short codes** — custom codes may contain simplified Chinese characters; extra reserved codes too (multi-segment entries reserve their whole subtree)
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
echo "SESSION_SECRET=$(openssl rand -hex 32)" > .env

# 3. Start
docker compose up -d
```

Open the web UI (the app listens on port 8080 unless `PORT` overrides it) —
with no password configured you land on the one-time setup page where the
first visitor sets the admin password (stored as a bcrypt hash in
`config.yaml`). After that it's the admin console.
Data (SQLite, uploaded icons, embedded Redis rdb, rotating logs) persists in
`./data`, config in `./config` (the settings page writes back to it).

First deployment needs nothing extra: the entrypoint runs as root only long
enough to chown the freshly created `./data` and `./config` bind mounts to
the unprivileged gourl user, then drops privileges (su-exec) for Redis and
gourl. The Redis "vm.overcommit_memory" warning is harmless in containers.

Images are built and published by GitHub Actions on [GHCR](https://github.com/wmy2981/gourl/pkgs/container/gourl):
`ghcr.io/wmy2981/gourl:latest` (releases, main branch) or `:dev` (pre-releases).
To deploy a pre-release: `GOURL_IMAGE=ghcr.io/wmy2981/gourl:dev docker compose up -d`.

Pre-release images embed the commit hash in the version string
(`0.1.0 (abc1234)`), visible in `/api/v1/health` and the admin footer, so a
running dev build identifies its exact commit; main builds keep the plain
version.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `ADMIN_PASSWORD` | — | Legacy env password. If set and `config.yaml` has no `password_hash`, it is migrated once — hashed and written back to the config file — and ignored afterwards. Prefer the setup flow. |
| `SESSION_SECRET` | insecure default | Signs admin session cookies — **set it in production** |
| `REDIS_ADDR` | `localhost:6379` | Click-counter buffer |
| `DB_PATH` | `data/gourl.db` | SQLite file |
| `PORT` | `8080` | HTTP listen port |
| `CONFIG_PATH` | `config.yaml` | Business config (site info, base URLs, …) |
| `ASSETS_DIR` | `data/assets` | Uploaded icon storage |
| `TZ` | container default | Daily click buckets and expiry are interpreted in it |
| `LOG_FORMAT` | `text` | `json` for structured output (logs go to stderr). The log **level** lives in `config.yaml` (`log_level`, settings page) — no env var |
| `LOG_DIR` | — | Optional directory for a rotating file mirror of the logs (e.g. `/app/data/log` on the mounted volume; 10 MB × 5 backups × 30 days, gzip) |

Business settings live in `config.yaml` (see `config.yaml.example`): site
name/title/keywords/description, random code length, primary + extra base
URLs, extra reserved codes, User-Agent block patterns, IP ban rules, the
login rate limit, the per-second link access budget, the admin `password_hash`
(set by the setup flow), and the custom icon.

## API documentation

Open `/docs/` on the running instance — an interactive Swagger UI covering
every endpoint, served from the single binary (`/docs/openapi.yaml` is the
raw OpenAPI 3.0 spec). API base path is `/api/v1`; admin endpoints accept a
session cookie or `Authorization: Bearer <token>` (tokens are created in
Settings).

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

## License

[MIT](LICENSE)
