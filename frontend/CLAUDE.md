# gourl frontend — Project Conventions

Subdirectory instructions for the React SPA admin console. Root-level conventions (commits, branch model, versioning) live in the repository-root `CLAUDE.md` and apply here too.

## Commands

- Install: `npm ci` (use the lockfile, never `npm install` for CI parity)
- Type check: `npm run typecheck` (`tsc -b`, strict mode, noUnusedLocals)
- Unit tests: `npm run test` (vitest, jsdom) — only `src/**/*.{test,spec}.{ts,tsx}` is collected; `e2e/` specs are Playwright's, never vitest's
- Build: `npm run build` (tsc + vite; recharts is chunked via `manualChunks` as a **function** — rolldown/vite 8 rejects the object form)
- E2E: `npm run e2e` (Playwright). The webServer auto-starts `go run ../cmd/e2e` (in-memory SQLite + miniredis, admin password `e2e-password`) — no external services needed. `npm run e2e:headed` for a visible browser
- E2E gotchas learned the hard way:
  - The e2e server serves the **embedded build** — after any `src/` change, run `powershell -File scripts/build-frontend.ps1` (repo root) first or tests run stale UI
  - `request` fixtures **follow redirects by default** — for 302 assertions pass `{ maxRedirects: 0 }` or you get the external target's status
  - Visibility assertions after list mutations need a generous timeout (15s) — the SPA refetch lags on slow runners; avoid asserting transient toasts
  - Codes like `expired`/`docs` are reserved — never use them as test fixtures
  - All specs share one server (in-memory DB) and run with `workers: 1`; per-run ports isolate parallel CI runs

## Gotchas

- **Icon single source**: the brand icon lives at the repo root `assets/favicon.svg`. `src/assets/icon.svg` is a **gitignored generated copy** (imported by `Layout.tsx`); never edit it — change the root source and re-copy (`cp ../../assets/favicon.svg src/assets/icon.svg`). CI and `scripts/build-frontend.ps1` regenerate it
- **Version injection**: `vite.config.ts` reads the repo-root `VERSION` file (`../VERSION`) into `__APP_VERSION__` — used in the footer and login page. The Docker build copies VERSION to `/VERSION` (filesystem root) because `../VERSION` resolves there from `/app`, and dev builds overwrite it with `VERSION (sha7)` (build arg `VERSION_STR`)
- **E2E data is shared**: all specs hit the same webServer process (single in-memory DB) and run serially (`workers: 1`). Tests must not depend on list ordering beyond the newest-first default
- **Playwright assertions**: `getByText` collides when both the code cell and the full short-URL line contain the same text — use `{ exact: true }` for short-code assertions, and scope dialog buttons via `getByRole('dialog')`
- **Row URL button collision**: each link row's base-URL picker button carries `aria-label={t('links.pickBaseUrl')}` — without it the accessible name is the URL itself and fuzzy role matchers like `getByRole('button', { name: /edit/i })` click it instead of the row's Edit action
- **Admin routes**: `/admin/*` is served by the Go binary's SPA fallback (not vite dev in CI builds). New system path prefixes must be added to `internal/shortcode` reserved codes (e.g. `docs`, `api`) so short codes can never shadow them
- **`/assets/` prefix is shared**: uploaded custom icons (`custom-icon.*`, served first) and vite build artifacts both live under `/assets/` server-side

## Design

- UI accent is **amber** on graphite neutrals (Apple-style glassmorphism); never default blue-purple gradients. The **brand icon is amber `#f59e0b`** too (user-specified, shares the accent color)
- Motion runs on **CSS tokens** (`--animate-*` in `index.css`) for pages/dialogs and on **`motion` (framer-motion)** for toasts: a stacked pile (newest in front, rear cards peek 12px, hover expands, spring 380/32), content-sized cards with measured heights. Do not reintroduce a global `prefers-reduced-motion` kill-switch — it was removed on purpose (the owner's OS has it enabled and wants the animations)
- The **mobile drawer slides via inline transform + transition** (gesture-aware, see Layout.tsx): open/close/gesture all share one `translateX` state machine, no class-animation path. Edge-swipe opens it from the right 24px of the viewport, fingers drag with clamping, release settles past the halfway point
- Every user-facing warning/error goes through the toast system — no inline error paragraphs, and no native browser validation bubbles: required-field checks are custom + toast (e.g. `form.urlRequired`)
- Form controls come from `ui.tsx`: `Select` (custom dropdown — never raw `<select>`; its options panel renders through a portal with **fixed positioning** — glass cards create stacking contexts via backdrop-blur, so an absolutely positioned panel inside a card gets covered by the next card — and plays the same `animate-pop-in`/`animate-pop-out` pair as the Links page's inline base-URL menu, every close path animating out before unmounting; keep `popOutMs` in sync with `--animate-pop-out`), `Checkbox` (drawn, never raw `<input type="checkbox">` — the OS palette clashes with both themes), `DateInput` (fixed yyyy/MM/dd + native picker via showPicker). All aria-labels must be i18n keys
- Dates render as `yyyy/MM/dd` (DateInput) and `yyyy/MM/dd HH:mm` (list columns, see `formatDate` in Links.tsx) — never `toLocaleString()`/native date-input display, whose format follows the browser locale. **Exception**: the dashboard trend-chart x-axis ticks are `MM/dd` (the tooltip keeps the full date)
- Dialogs: `Dialog` accepts `headerActions` (buttons rendered next to the close button — the QR dialog's JPEG download uses it). Dialogs lock page scroll while open (built into `Dialog`) — don't add per-dialog scroll locks
- Batch create (`BatchCreateDialog` + `lib/batch.ts`) parses strict lines `[code](date)url`, flags format errors before submit, and keeps failed lines editable for the retry button (server-side `code_taken` etc.)
- The log page (`pages/Logs.tsx`) streams via `EventSource` (`api.logStream`), auto-reconnects, and renders history oldest-first; attrs are hidden below `sm` so messages never collapse on phones
- Scrollbars are themed globally in `index.css` (webkit pseudo-elements + Firefox `scrollbar-color`); don't restyle per-container
- **Short URLs are assembled client-side**: `linkUrls(code, cfg)` in `lib/api.ts` mirrors the old backend fullURLs — base_url (fallback `location.protocol//location.host`) plus extra_base_urls, trailing slashes trimmed, deduplicated. The backend link payload carries only `code` + `id`; QRDialog receives the assembled urls as a prop from Links. The link form's http/https buttons use `applyScheme` (fill / swap / leave alone)
- Follow the frontend-design skill's two-pass process (token system first, then implementation) for UI work
- All copy is bilingual via `react-i18next`; `src/locales/en.json` and `zh.json` must keep identical key sets (enforced by a vitest test)
- Forms must associate `<Label htmlFor>` with input `id` — accessibility and Playwright's `getByLabel` depend on it
- Settings lists (reserved codes, UA blocks, IP blocks) are **comma-separated** textareas applied by the save button; extra base URLs stay newline-separated; the log level is a `Select` with the four levels
- Auth pages: `/admin/login` and `/admin/setup` both fetch the service name from the **public health endpoint** (`health.name`, fallback `__APP_NAME__`) — the config API refuses requests while no admin password exists. The browser tab shows the service name there, the site title in the console (`App.tsx`, keyed on `location.pathname`). `App` routes unconfigured servers (`authStatus.configured === false`) to the setup page and back once configured; `/admin` itself redirects to `/admin/setup` while unconfigured
- List layout: the title renders **under the destination URL** (destination cell), not in the short-code cell; table headers are `whitespace-nowrap`
