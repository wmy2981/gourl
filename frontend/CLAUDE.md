# gourl frontend — Project Conventions

Subdirectory conventions for the React SPA admin. Root-level conventions (commits, branch model, versioning) live in the repository-root `CLAUDE.md` and apply here too.

## Commands

- Install: `npm ci` (lockfile only — never `npm install`)
- Check/test/build scripts are in package.json; vitest collects only `src/**/*.{test,spec}.{ts,tsx}` (e2e/ is Playwright's)
- E2E: `npm run e2e` — Playwright, `workers: 1`, all specs share one webServer process (in-memory DB, admin password `e2e-password`); port `8099` locally, per-run from `GITHUB_RUN_ID` in CI

## E2E gotchas

- The e2e server serves the **embedded build** — after any `src/` change run `powershell -File scripts/build-frontend.ps1` (repo root) first or tests run stale UI
- `request` fixtures **follow redirects by default** — pass `{ maxRedirects: 0 }` for 302 assertions
- Visibility assertions after list mutations need ~15s timeouts (the SPA refetch lags on slow runners); don't assert transient toasts
- Codes like `expired`/`docs` are reserved — never use them as test fixtures
- `getByText` collides when a code cell and the full short-URL line share text — use `{ exact: true }`; scope dialog buttons via `getByRole('dialog')`

## Gotchas

- **Icon single source**: `src/assets/icon.svg` is a gitignored copy of root `assets/favicon.svg` — never edit it; CI and build scripts regenerate it. Android launcher bitmaps render from the same file via `scripts/sync-android-icon.mjs`
- **Version injection**: vite.config.ts reads the root `VERSION` into `__APP_VERSION__` (footer/login). In Docker, `../VERSION` resolves to `/VERSION` at the filesystem root (the build copies it there)
- Each row's base-URL picker button needs `aria-label={t('links.pickBaseUrl')}` — without it the accessible name is the URL itself and fuzzy role matchers like `getByRole('button', { name: /edit/i })` hit it instead of the row's Edit action
- New system path prefixes must join `internal/shortcode` reserved codes or short codes can shadow them
- `/assets/` is shared: uploaded custom icons (`custom-icon.*`, served first) and vite build artifacts live under the same prefix

## Design conventions

- Accent is **amber** on graphite (Apple-style glassmorphism); the brand icon is amber `#f59e0b`; never default blue-purple gradients
- Theme cycles light → dark → **system** (`gourl-theme` in localStorage, default system): `'system'` and unset are identical (pre-paint scripts in index.html + App.tsx)
- Every user-facing warning/error goes through **toasts only** — no inline error paragraphs, no native browser validation bubbles
- Controls from `ui.tsx`, never native: `Select`, `Checkbox`, `DateInput`. All aria-labels must be i18n keys. The custom `Select` renders its options panel through a portal with **fixed positioning** — glass-card `backdrop-blur` creates a containing block that traps absolute panels behind the next card
- Dates render `yyyy/MM/dd` (+ `HH:mm` via `formatDate`) — never `toLocaleString()`/native locale display. Exception: dashboard trend x-ticks are `MM/dd`
- **Dialogs must render at page level, never inside a glass card**: backdrop-blur traps `fixed inset-0` descendants in the card's box, clipping the panel header (comments in Settings.tsx enforce this). `Dialog` locks page scroll and **snapshots its content during the close animation** — parents null their data in the closing render, so keep that snapshot mechanism intact
- The QR download paints a fixed-light branded card (readable on any paper/theme). **The header JPEG download button is hidden on purpose** (2026-08) — the handler and its test stay intact so re-enabling stays a one-word diff; don't delete them as dead code
- Motion: CSS tokens for pages/dialogs, `motion` (framer-motion) for toasts (stacked pile, newest in front). **No global `prefers-reduced-motion` kill-switch — removed on purpose** (the owner's OS has it enabled and wants the animations)
- Mobile drawer: one `translateX` state machine; edge-swipe opens from anywhere in the **right 30%** of the viewport (phones only); touches starting inside a `table` never trigger it
- Scrollbars are themed globally in `index.css` — don't restyle per-container
- Copy is bilingual via `react-i18next`: `en.json`/`zh.json` must keep identical key sets (enforced by a vitest test)
- Forms must associate `<Label htmlFor>` with input `id` — accessibility and Playwright's `getByLabel` depend on it
- Auth pages fetch the service name from the **public health endpoint** — the config API refuses requests while no admin password exists. `App` routes unconfigured servers (`authStatus.configured === false`) to `/admin/setup` and back once configured
- Short URLs are assembled **client-side** (`linkUrls(code, cfg)` in lib/api.ts mirrors base_url + extra_base_urls logic) — the backend link payload carries only `code` + `id`
- Batch create validation runs **on editor blur or Create click — never on every keystroke** (`CodeEditor.onBlur`, jsdom needs the focus-timer settle in tests); keeps failed lines editable for the retry button
- Import accepts both the legacy bare array and the export dump (`{meta, items}` — unwrap `.items`); CSV import skips the exporter's `#`-comment metadata lines before parsing
- jsdom lacks DOM APIs CodeMirror needs — shared stubs live in `src/test/setup.ts` (ResizeObserver + Range client rects); CodeMirror dispatches focus changes on a ~10ms timer, so blur tests must settle between focus and blur
- Log page: SSE via `api.logStream`, auto-reconnects, history oldest-first; attrs are hidden below `sm` so messages never collapse on phones
- Follow the frontend-design skill's two-pass process (token system first, then implementation)

## Mobile app (Capacitor, `android/`)

- **Token mode**: stored `{url, token}` in `localStorage.gourl-server`, absolute-URL requests with a `Bearer` header + `credentials: 'omit'`; a 401 **never** redirects to login/setup; an unconnected app lands on `/admin/connect` (probe-before-persist). The sidebar logout button is hidden — the settings disconnect card owns that flow
- Downloads via `lib/download.ts` (`saveDownload`, used by QRDialog + ExportDialog + the log export): `<a download>` on web; in the app, system **Downloads/gourl/** through the native bridge (`GourlBridge.saveToDownloads` in MainActivity). Filenames come from `exportFilename(kind, ext)` matching the backend's shape
- Log stream: fetch-parsed SSE with the Bearer header in app mode (EventSource cannot send one); plain `EventSource` on web
- Back button (App.tsx): Escape to an open `[role="dialog"]` first, then `exitApp()`; `gourl://dashboard|links|log|settings` deep links navigate the SPA
- System bars: `html.capacitor` zeroes body padding (the WebView never reports the inset); top offsets come from `<main>`'s padding and `.mobile-topbar`'s own padding — keep enough top white-space to clear the status bar
- **Connection card** (`ConnectionCard`, Settings.tsx) always renders, independent of the config loading state; probes `api.getConfig()` every 10s + on app foreground: success → connected, 401 → unauthorized, network error → unreachable
- API-docs link resolves against the server URL and opens with `_system`; grab `window.Capacitor.Plugins.App` for `appStateChange` like App.tsx — never import `@capacitor/app` directly in web-consumed code
