# Builds the frontend and copies artifacts into the Go embed locations.
# Run before `go build ./cmd/gourl` or `go test ./...`.
$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot\.."

# Single source of truth for the brand icon: assets/favicon.svg. Copy it to
# the places the frontend (vite import) and backend (go:embed) need it.
# (mkdir: a clean checkout has neither src/assets nor any generated embed file.)
New-Item -ItemType Directory -Force -Path "frontend\src\assets", "internal\webui" | Out-Null
Copy-Item "assets\favicon.svg" "frontend\src\assets\icon.svg" -Force
Copy-Item "assets\favicon.svg" "internal\webui\icon.svg" -Force

# Locale files are the single source for backend-rendered page copy
# (404 / blocked pages) — copy them next to the go:embed inputs.
$locales = "$PSScriptRoot\..\internal\webui\locales"
if (Test-Path $locales) { Remove-Item $locales -Recurse -Force }
New-Item -ItemType Directory -Path $locales | Out-Null
Copy-Item "frontend\src\locales\en.json" "$locales\en.json"
Copy-Item "frontend\src\locales\zh.json" "$locales\zh.json"

Set-Location frontend
npm ci --no-audit --no-fund
npm run build
$dist = "$PSScriptRoot\..\internal\webui\dist"
if (Test-Path $dist) { Remove-Item $dist -Recurse -Force }
Copy-Item -Recurse "dist" "$dist"
Write-Host "frontend artifacts copied to internal/webui/dist"
