/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { readFileSync } from 'node:fs'

// Version injected from the repo-root VERSION file (single source of truth),
// shown in the admin footer and the login page.
const version = readFileSync(new URL('../VERSION', import.meta.url), 'utf-8').trim()

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
        // rolldown (vite 8) accepts manualChunks as a function only.
        manualChunks(id: string) {
          if (id.includes('/recharts/') || id.includes('/d3-') || id.includes('/victory-vendor/')) {
            return 'charts'
          }
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  },
})
