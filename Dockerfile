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
RUN npm run build

# ---------- go build ----------
FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Frontend artifacts land in the embed location (relative to the webui package).
COPY --from=frontend /app/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/wmy2981/gourl/internal/version.Version=$(cat VERSION)" \
    -o /gourl ./cmd/gourl

# ---------- runtime ----------
FROM alpine:3.21
RUN adduser -D -u 10001 gourl
USER gourl
WORKDIR /app
COPY --from=build /gourl /usr/local/bin/gourl

# Mount points: SQLite database + uploaded icons (data/), business config.
VOLUME ["/app/data", "/app/config.yaml"]

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["gourl"]
