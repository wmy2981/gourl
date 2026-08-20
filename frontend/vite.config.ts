/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { readFileSync } from 'node:fs'

// Version injected from the repo-root VERSION file (single source of truth),
// shown in the admin footer and the login page. A VERSION_STR environment
// variable overrides it — mirroring the Docker build ARG, the build scripts
// and the Android workflow set it to "VERSION (sha7)" on non-main branches
// so every artifact identifies its exact build.
const version = (process.env.VERSION_STR ?? readFileSync(new URL('../VERSION', import.meta.url), 'utf-8')).trim()

export default defineConfig({
  plugins: [react(), tailwindcss()],
  define: {
    __APP_VERSION__: JSON.stringify(version),
    __APP_NAME__: JSON.stringify('gourl'),
    __APP_REPO__: JSON.stringify('https://github.com/wmy2981/gourl'),
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          charts: ['recharts'],
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    // Playwright specs live in e2e/ and must not be collected by vitest.
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
})
