# gourl

> [中文文档](README.zh-CN.md) | English

<p align="left">
  <img src="assets/favicon.svg" width="72" height="72" alt="gourl" />
</p>

Lightweight self-hosted URL shortener. A single Go binary (frontend embedded)
plus Redis — that's all it takes to run your own short links.

## Features

- **Short links** — auto-generated or custom codes, multi-level paths, simplified Chinese supported; per-link expiry and QR-code download
- **Admin console** — responsive, glassmorphism UI with light/dark/system themes and English/Chinese; REST API with bearer tokens and Swagger UI at `/docs/`
- **Click stats** — buffered in Redis and flushed to SQLite every 30s; history survives link deletion (soft deletes only)
- **Auto titles** — async `title`/`description` fetching for any reachable host, internal networks included
- **Security** — setup requires a one-time bootstrap code from the server log; bcrypt admin password, configurable session expiry, per-IP login lockout, UA/IP blocking, self-link guard
- **Batch & import/export** — bulk create/delete, clear-expired, lenient CSV/JSON import with conflict policies, full JSON export
- **Edit snapshots** — every edit appends an immutable backup (`b-1, b-2, …`)
- **Container CLI** — `gourl reset …`, `gourl db export`, `gourl status`, `gourl webui on|off`, `gourl restart` and more, inside the container
- **Structured logging** — slog with 4 levels, mirrored to a rotating file, live log page via SSE

## Stack

Go (stdlib `net/http`, `log/slog`) · SQLite ([modernc](https://modernc.org/sqlite), no CGO) ·
Redis · React 19 + Vite + Tailwind CSS 4 + shadcn-style components

## Quick start (Docker)

Single container — the image embeds a Redis instance, so one `compose up`
is the whole deployment.

```bash
# 1. Optional: copy config.yaml.example to config/config.yaml and adjust.
# 2. Create .env next to docker-compose.yml:
echo "SESSION_SECRET=$(openssl rand -hex 32)" > .env

# 3. Start
docker compose up -d
```

Open the web UI on port 8080 — with no password configured you land on the
setup page (enter the code from the server log, then set the admin password).
Data lives in `./data`, config in `./config`.

Images are published on [GHCR](https://github.com/wmy2981/gourl/pkgs/container/gourl):
`ghcr.io/wmy2981/gourl:latest` (releases) or `:dev` (pre-releases). Run the CLI
with `docker compose exec app gourl <command>`.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `SESSION_SECRET` | auto-generated | Signs admin sessions — set it so sessions survive restarts |
| `PORT` | `8080` | HTTP listen port |
| `DB_PATH` | `./data/gourl.db` | SQLite file |
| `REDIS_ADDR` | `localhost:6379` | Click-counter buffer (embedded Redis when unset) |
| `CONFIG_PATH` | `./config/config.yaml` | Business config file |
| `LOG_DIR` | `./data/log` | Rotating log mirror |

Business settings (site info, base URLs, reserved codes, rate limits, log
level, …) live in `config.yaml` — see `config.yaml.example`.

## API documentation

Open `/docs/` on the running instance for the interactive Swagger UI.
API base path is `/api/v1`; admin endpoints accept a session cookie or
`Authorization: Bearer <token>` (tokens are created in Settings).

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
# POSIX shells: ./scripts/build-frontend.sh && go build ./cmd/gourl
```

## License

[MIT](LICENSE)
