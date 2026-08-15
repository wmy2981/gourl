# syntax=docker/dockerfile:1

# ---------- frontend build ----------
FROM node:22-alpine AS frontend
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
# vite.config.ts resolves ../VERSION relative to the config file (/app),
# which lands on the filesystem root.
COPY VERSION /VERSION
COPY frontend/ ./
# Brand icon: single source of truth is assets/favicon.svg; the frontend
# imports it from src/assets/icon.svg (generated location).
COPY assets/favicon.svg ./src/assets/icon.svg
RUN npm run build

# ---------- go build ----------
FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Frontend artifacts land in the embed location (relative to the webui package).
COPY --from=frontend /app/dist ./internal/webui/dist
# Brand icon for the Go embed (generated location, single source assets/favicon.svg).
COPY assets/favicon.svg ./internal/webui/icon.svg
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/wmy2981/gourl/internal/version.Version=$(cat VERSION)" \
    -o /gourl ./cmd/gourl

# ---------- runtime ----------
FROM alpine:3.21
# redis-server for single-container deployments (embedded Redis on
# 127.0.0.1:6379 unless REDIS_ADDR points elsewhere).
RUN apk add --no-cache redis && adduser -D -u 10001 gourl
# entrypoint.sh: start the embedded Redis unless REDIS_ADDR is set, then run
# gourl. chmod runs as root (before USER) because the binary dir is root-owned
# and git mode bits are unreliable across platforms.
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
USER gourl
WORKDIR /app
COPY --from=build /gourl /usr/local/bin/gourl

# Mount points: SQLite database + uploaded icons + embedded Redis rdb (data/),
# business config directory.
VOLUME ["/app/data", "/app/config"]

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
