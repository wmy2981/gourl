#!/usr/bin/env sh
# Builds the frontend and copies artifacts into the Go embed locations.
# Run before `go build ./cmd/gourl` or `go test ./...` (POSIX shells, CI).
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Single source of truth for the brand icon: assets/favicon.svg. Copy it to
# the places the frontend (vite import) and backend (go:embed) need it.
cp -f "$ROOT/assets/favicon.svg" "$ROOT/frontend/src/assets/icon.svg"
cp -f "$ROOT/assets/favicon.svg" "$ROOT/internal/webui/icon.svg"

cd "$ROOT/frontend"
npm ci --no-audit --no-fund
npm run build
dist="$ROOT/internal/webui/dist"
rm -rf "$dist"
cp -R dist "$dist"
echo "frontend artifacts copied to internal/webui/dist"
