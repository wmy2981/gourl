# Builds the frontend and copies the artifacts into web/dist, where the Go
# binary embeds them. Run before `go build ./cmd/gourl`.
$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot\..\frontend"
npm ci --no-audit --no-fund
npm run build
# embed paths are relative to the Go package, so the artifacts land in
# internal/webui/dist (embed cannot reach "..").
$dist = "$PSScriptRoot\..\internal\webui\dist"
if (Test-Path $dist) { Remove-Item $dist -Recurse -Force }
Copy-Item -Recurse "dist" "$dist"
Write-Host "frontend artifacts copied to internal/webui/dist"
