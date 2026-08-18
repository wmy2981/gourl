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

- **Icon single source**: `src/assets/icon.svg` is a gitignored copy of root `assets/favicon.svg` — never edit it (`cp ../../assets/favicon.svg src/assets/icon.svg`; CI and build scripts regenerate). Android launcher bitmaps render from the same file via `scripts/sync-android-icon.mjs`. **No adaptive-icon resources** — the platform scales/masks legacy mipmaps itself; hand-written VectorDrawable transforms landed the glyph off-center or invisible
- **Version injection**: vite.config.ts reads the root `VERSION` into `__APP_VERSION__` (footer/login). In Docker, `../VERSION` resolves to `/VERSION` at the filesystem root (the build copies it there); dev builds get `VERSION (sha7)`
- Each row's base-URL picker button needs `aria-label={t('links.pickBaseUrl')}` — without it the accessible name is the URL itself and fuzzy role matchers like `getByRole('button', { name: /edit/i })` hit it instead of the row's Edit action
- New system path prefixes must join `internal/shortcode` reserved codes (e.g. `docs`, `api`) or short codes can shadow them
- `/assets/` is shared: uploaded custom icons (`custom-icon.*`, served first) and vite build artifacts live under the same prefix

## Design conventions

- Accent is **amber** on graphite (Apple-style glassmorphism); the brand icon is amber `#f59e0b` (user-specified, shares the accent); never default blue-purple gradients
- Theme cycles light → dark → **system** (`gourl-theme` in localStorage, default system, `Monitor` icon marks it): only an explicit `'light'` forces light; `'system'` and unset are identical (pre-paint scripts in index.html + App.tsx)
- Every user-facing warning/error goes through **toasts only** — no inline error paragraphs, no native browser validation bubbles (required-field checks are custom, e.g. `form.urlRequired`)
- Controls from `ui.tsx`, never native: `Select` (custom dropdown — the options panel renders through a portal with **fixed positioning**: glass-card `backdrop-blur` creates a containing block that traps absolute panels behind the next card; every close path animates out via `animate-pop-out` — keep `popOutMs` in sync with `--animate-pop-out`), `Checkbox` (drawn — the OS palette clashes with both themes), `DateInput` (fixed yyyy/MM/dd + native picker via `showPicker`). All aria-labels must be i18n keys
- Dates render `yyyy/MM/dd` (DateInput) and `yyyy/MM/dd HH:mm` (`formatDate` in Links.tsx) — never `toLocaleString()`/native date display (browser locale). Exception: dashboard trend x-ticks are `MM/dd`, the tooltip keeps the full date
- **Dialogs must render at page level, never inside a glass card**: `backdrop-blur` traps `fixed inset-0` descendants in the card's box — the panel overflows the card and its header gets clipped (the disconnect-confirm dialog hit this; two comments in Settings.tsx enforce it). `Dialog` locks page scroll, accepts `headerActions` (the QR JPEG download uses it), and **snapshots its content during the close animation** — parents null their data in the closing render, so keep that snapshot mechanism intact
- The QR download paints a **fixed-light branded card** (readable on any paper/theme); `roundRect` requires modern browsers. The header JPEG download button is `hidden` (2026-08) — the `download` handler and its test stay intact, so re-enabling is a one-word diff
- Motion: CSS tokens (`--animate-*` in index.css) for pages/dialogs, `motion` (framer-motion) for toasts (stacked pile, newest in front, rear cards peek 12px). **No global `prefers-reduced-motion` kill-switch — removed on purpose** (the owner's OS has it enabled and wants the animations)
- Mobile drawer: one `translateX` state machine for open/close/gesture (no class-animation path); edge-swipe opens from anywhere in the **right 30%** of the viewport (phones only); touches starting inside a `table` never trigger it (wide tables keep horizontal scrolling)
- Scrollbars are themed globally in `index.css` — don't restyle per-container
- Copy is bilingual via `react-i18next`: `en.json`/`zh.json` must keep identical key sets (enforced by a vitest test)
- Forms must associate `<Label htmlFor>` with input `id` — accessibility and Playwright's `getByLabel` depend on it
- Settings lists (reserved codes, UA blocks, IP blocks) are **comma-separated** textareas applied by the save button; extra base URLs stay newline-separated; the log level is a `Select` with the four levels
- Auth pages fetch the service name from the **public health endpoint** (`health.name`, fallback `__APP_NAME__`) — the config API refuses requests while no admin password exists; the browser tab shows it there, the console title is keyed on `location.pathname`. `App` routes unconfigured servers (`authStatus.configured === false`) to `/admin/setup` and back once configured
- Short URLs are assembled **client-side**: `linkUrls(code, cfg)` in lib/api.ts mirrors the old backend fullURLs (base_url with `location` fallback + extra_base_urls, trailing slashes trimmed, deduped) — the backend link payload carries only `code` + `id`; the form's http/https buttons use `applyScheme` (fill / swap / leave alone)
- Batch create (`BatchCreateDialog` + `lib/batch.ts`): strict lines `[code](date)url`, flags format errors before submit, keeps failed lines editable for the retry button
- Log page: SSE via `api.logStream`, auto-reconnects, history oldest-first; attrs are hidden below `sm` so messages never collapse on phones
- List layout: the title renders **under the destination URL** (destination cell), not in the short-code cell; table headers are `whitespace-nowrap`
- Follow the frontend-design skill's two-pass process (token system first, then implementation)

## Mobile app (Capacitor, `android/`)

- **Token mode**: stored `{url, token}` in `localStorage.gourl-server` (`getServerConfig`/`setServerConfig` in lib/api.ts), absolute-URL requests with a `Bearer` header + `credentials: 'omit'`; a 401 **never** redirects to login/setup; an unconnected app lands on `/admin/connect` (probe-before-persist). The sidebar logout button is hidden — the settings disconnect card owns that flow
- Downloads via `lib/download.ts` (`saveDownload`, used by QRDialog + ExportDialog): `<a download>` on web; in the app, system **Downloads/gourl/** via `@capgo/capacitor-file-sharer` with the path toasted — never `@capacitor/filesystem` (its v8 enum lost the Downloads directory)
- Log stream: fetch-parsed SSE with the Bearer header in app mode (EventSource cannot send one); plain `EventSource` on web
- Back button (App.tsx): Escape to an open `[role="dialog"]` first, then `exitApp()`; `gourl://dashboard|links|log|settings` deep links navigate the SPA
- System bars: the WebView never reports the inset — `html.capacitor` only zeroes body padding (the native side stopped padding the WebView; `--safe-area-inset-*` has no consumers); top offsets come from `<main>`'s `pt-16` and `.mobile-topbar`'s own padding (`pt-6` since 2026-08, doubled from `pt-3` — keep the top white-space at least that large so the bar clears the status bar)
- **Connection card** (`ConnectionCard`, Settings.tsx) always renders, independent of the config loading state; probes `api.getConfig()` (`GET /api/v1/config`, behind `requireAuth`) every 10s + on app foreground: success → connected, 401 → unauthorized, network error → unreachable
- API-docs link resolves against the server URL and opens with `_system`; grab `window.Capacitor.Plugins.App` for `appStateChange` like App.tsx — never import `@capacitor/app` directly in web-consumed code
