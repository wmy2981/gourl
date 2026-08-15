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
# The brand icon lives in the frontend source; keep the embedded copy in sync.
Copy-Item "src\assets\icon.svg" "$PSScriptRoot\..\internal\webui\icon.svg" -Force
Write-Host "frontend artifacts copied to internal/webui/dist"
