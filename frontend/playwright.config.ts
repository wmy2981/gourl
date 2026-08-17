import { defineConfig } from '@playwright/test'

// End-to-end tests run against cmd/e2e (in-memory SQLite + miniredis), so
// no external Redis or database is required. The server is started and
// stopped automatically by Playwright.
//
// E2E_PORT isolates parallel CI runs: each run gets a unique port so two
// concurrent e2e jobs never share (and pollute) the same server. On GitHub
// Actions the run id (injected as GITHUB_RUN_ID) picks a port in [20000,
// 60000); locally the fixed 8099 is kept.
const port =
  process.env.E2E_PORT ??
  (process.env.GITHUB_RUN_ID
    ? String(20000 + (Number(process.env.GITHUB_RUN_ID.slice(-4)) % 40000))
    : '8099')
// cmd/e2e listens on E2E_PORT from its own environment; the derived port must
// be visible to the webServer child process (and to `go run ../cmd/e2e`).
process.env.E2E_PORT = port

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  fullyParallel: false,
  workers: 1,
  reporter: [['list']],
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    locale: 'en-US',
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'go run ../cmd/e2e',
    url: `http://127.0.0.1:${port}/api/v1/health`,
    reuseExistingServer: true,
    timeout: 30_000,
  },
})
