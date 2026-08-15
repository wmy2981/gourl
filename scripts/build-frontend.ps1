# Builds the frontend and copies artifacts into the Go embed locations.
# Run before `go build ./cmd/gourl` or `go test ./...`.
$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot\.."

# Single source of truth for the brand icon: assets/favicon.svg. Copy it to
# the places the frontend (vite import) and backend (go:embed) need it.
Copy-Item "assets\favicon.svg" "frontend\src\assets\icon.svg" -Force
Copy-Item "assets\favicon.svg" "internal\webui\icon.svg" -Force

Set-Location frontend
npm ci --no-audit --no-fund
npm run build
$dist = "$PSScriptRoot\..\internal\webui\dist"
if (Test-Path $dist) { Remove-Item $dist -Recurse -Force }
Copy-Item -Recurse "dist" "$dist"
Write-Host "frontend artifacts copied to internal/webui/dist"
