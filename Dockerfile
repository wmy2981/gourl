# syntax=docker/dockerfile:1

# ---------- frontend build ----------
# $BUILDPLATFORM: the JS bundle is platform-independent — always build on the
# native runner arch. Emulating npm/V8 under QEMU for arm64 intermittently
# dies with SIGILL and leaves the whole build hanging.
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
ARG VERSION_STR
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
# vite.config.ts resolves ../VERSION relative to the config file (/app),
# which lands on the filesystem root.
COPY VERSION /VERSION
# Dev images inject "VERSION (sha7)" into the file vite reads, so the footer
# and login page identify the exact build; local builds keep the plain file.
RUN if [ -n "$VERSION_STR" ]; then echo "$VERSION_STR" > /VERSION; fi
COPY frontend/ ./
# Brand icon: single source of truth is assets/favicon.svg; the frontend
# imports it from src/assets/icon.svg (generated location).
COPY assets/favicon.svg ./src/assets/icon.svg
RUN npm run build

# ---------- go build ----------
# $BUILDPLATFORM + GOARCH cross-compile: CGO is off, so the toolchain targets
# arm64 natively on the amd64 runner — no QEMU anywhere in the build.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG VERSION_STR
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
# One retry: the module proxy intermittently resets connections on runners.
RUN go mod download || (sleep 3 && go mod download)
COPY . .
# Frontend artifacts land in the embed location (relative to the webui package).
COPY --from=frontend /app/dist ./internal/webui/dist
# Brand icon for the Go embed (generated location, single source assets/favicon.svg).
COPY assets/favicon.svg ./internal/webui/icon.svg
# Locale files feed the backend-rendered page copy (404 / blocked pages) —
# same generated embed location the local build script uses.
COPY frontend/src/locales/ ./internal/webui/locales/
# The version string (quoted — dev builds carry "VERSION (sha7)") comes from
# the build ARG; without one (local builds) it falls back to the VERSION file.
RUN CGO_ENABLED=0 GOARCH=$TARGETARCH go build -trimpath \
    -ldflags "-s -w -X 'github.com/wmy2981/gourl/internal/version.Version=${VERSION_STR:-$(cat VERSION)}'" \
    -o /gourl ./cmd/gourl

# ---------- runtime ----------
FROM alpine:3.21
# redis-server for single-container deployments (embedded Redis on
# 127.0.0.1:6379 unless REDIS_ADDR points elsewhere). su-exec lets the
# entrypoint fix mount-point ownership as root, then drop privileges.
RUN apk add --no-cache redis tzdata su-exec && adduser -D -u 10001 gourl
# Default timezone for daily click buckets and expiry; compose overrides it
# with TZ from its .env. tzdata makes the zone resolvable in the image.
ENV TZ=Asia/Shanghai
# entrypoint.sh runs as root (no USER directive below) so freshly created
# bind mounts can be chowned to gourl, then Redis and gourl both run via
# su-exec as the unprivileged user.
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
WORKDIR /app
COPY --from=build /gourl /usr/local/bin/gourl

# Mount points: SQLite database + uploaded icons + embedded Redis rdb (data/),
# business config directory.
VOLUME ["/app/data", "/app/config"]

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
