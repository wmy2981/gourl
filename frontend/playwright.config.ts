import { defineConfig } from '@playwright/test'

// End-to-end tests run against cmd/e2e (in-memory SQLite + miniredis), so
// no external Redis or database is required. The server is started and
// stopped automatically by Playwright.
export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  fullyParallel: false,
  workers: 1,
  reporter: [['list']],
  use: {
    baseURL: 'http://127.0.0.1:8099',
    locale: 'en-US',
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'go run ../cmd/e2e',
    url: 'http://127.0.0.1:8099/api/v1/health',
    reuseExistingServer: true,
    timeout: 30_000,
  },
})
