# gourl — Project Conventions

## Overview

Lightweight self-hosted URL shortener. Go backend (stdlib `net/http`, no framework) + React 19/TS SPA admin (Vite + Tailwind 4 + shadcn/ui). SQLite stores links and related records only; business config lives in `config.yaml`; runtime/secrets come from env vars. Redis buffers click counts (30s batch flush).

The full design is in [DESIGN.md](DESIGN.md) (Chinese). **It is the single source of truth for architecture decisions — read it before implementing any feature.**

## Commands

- Build: `go build ./cmd/gourl`
- Test: `go test ./...` (Go tests use miniredis, no real Redis needed)
- Vet: `go vet ./...`
- Frontend: `cd frontend && npm ci && npm run build` (tsc + vite), `npm test` (vitest), Playwright E2E in `frontend/e2e`

## Git Workflow

- Conventional Commits, English, imperative mood: `type(scope): lowercase description`
- **One logical change = one commit**; each change must carry its tests, and nothing is committed until all tests pass
- Branch model: `main` (release) + `dev` (pre-release); feature branches merge via PR
- Version: single source of truth is the root `VERSION` file, maintained manually; release pipeline validates forward-only progression (see `.github/scripts/release_check.py`)

## Engineering Rules

- Go: stdlib only for HTTP; `modernc.org/sqlite` (no CGO — multi-platform builds); gofmt + go vet clean
- All user-facing strings (API errors, UI copy) are bilingual zh/en; site info fields in `config.yaml` are single-language
- No local Docker builds or deployments — images are built and pushed to GHCR (`ghcr.io/wmy2981/gourl`) exclusively by GitHub Actions workflows
- Follow the frontend-design skill's two-pass process for UI work (no default blue-purple gradients)
