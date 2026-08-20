#!/usr/bin/env sh
# Builds the frontend and copies artifacts into the Go embed locations.
# Run before `go build ./cmd/gourl` or `go test ./...` (POSIX shells, CI).
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Single source of truth for the brand icon: assets/favicon.svg. Copy it to
# the places the frontend (vite import) and backend (go:embed) need it.
# (mkdir: a clean checkout has neither src/assets nor any generated embed file.)
mkdir -p "$ROOT/frontend/src/assets" "$ROOT/internal/webui"
cp -f "$ROOT/assets/favicon.svg" "$ROOT/frontend/src/assets/icon.svg"
cp -f "$ROOT/assets/favicon.svg" "$ROOT/internal/webui/icon.svg"

# Locale files are the single source for backend-rendered page copy
# (404 / blocked pages) — copy them next to the go:embed inputs.
locales="$ROOT/internal/webui/locales"
rm -rf "$locales"
mkdir -p "$locales"
cp -f "$ROOT/frontend/src/locales/en.json" "$locales/en.json"
cp -f "$ROOT/frontend/src/locales/zh.json" "$locales/zh.json"

# Version string mirrors the Docker build: plain VERSION on main, "VERSION
# (sha7)" on any other branch — the same identity the image and APK carry.
# A failed git probe falls back to the plain file (like a Docker build
# without the VERSION_STR ARG).
version_str="$(cat "$ROOT/VERSION")"
if [ "$(git -C "$ROOT" branch --show-current 2>/dev/null)" != "main" ]; then
  sha="$(git -C "$ROOT" rev-parse --short=7 HEAD 2>/dev/null || true)"
  if [ -n "$sha" ]; then version_str="$version_str ($sha)"; fi
fi
export VERSION_STR="$version_str"

cd "$ROOT/frontend"
npm ci --no-audit --no-fund
npm run build
dist="$ROOT/internal/webui/dist"
rm -rf "$dist"
cp -R dist "$dist"
echo "frontend artifacts copied to internal/webui/dist"
