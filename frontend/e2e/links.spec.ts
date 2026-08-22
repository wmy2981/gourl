import { expect, test } from '@playwright/test'
import { apiLogin, createLinkApi, createLinkUi, login } from './helpers'

test.beforeEach(async ({ page, request }) => {
  await apiLogin(request)
  await login(page)
})

test('creates a custom-code link and shows full short urls', async ({ page }) => {
  await createLinkUi(page, 'https://example.com/very/long/path', 'e2e-custom')
  await expect(page.getByText('e2e-custom', { exact: true })).toBeVisible({ timeout: 15_000 })
  // The short URL is derived from the request host (no base_url configured);
  // its origin matches the page's, whatever port the run was assigned.
  const origin = new URL(page.url()).origin
  // The list refresh after creation can lag on slow runners.
  await expect(page.getByText(`${origin}/e2e-custom`).first()).toBeVisible({ timeout: 15_000 })
})

test('creates a multi-level code', async ({ page }) => {
  await createLinkUi(page, 'https://example.com/deep', 'guide/part1')
  await expect(page.getByText('guide/part1', { exact: true })).toBeVisible({ timeout: 15_000 })
})

test('rejects a reserved code with a friendly error', async ({ page }) => {
  await page.goto('/admin/links')
  await page.getByRole('button', { name: /new link/i }).click()
  await page.getByLabel('Destination URL').fill('https://example.com/x')
  await page.getByLabel(/Short code/).fill('api')
  await page.getByRole('button', { name: /^Save$/ }).click()
  await expect(page.getByText('This short code is reserved')).toBeVisible({ timeout: 15_000 })
})

test('rejects a duplicate code', async ({ page, request }) => {
  await createLinkApi(request, { url: 'https://example.com/taken', code: 'taken-code' })
  await createLinkUi(page, 'https://example.com/other', 'taken-code')
  await expect(page.getByText('This short code is already in use')).toBeVisible({ timeout: 15_000 })
})

test('batch imports links and reports per-item results', async ({ page }) => {
  await page.goto('/admin/links')
  await page.getByRole('button', { name: /import/i }).click()
  // The CodeMirror editor replaces the textarea: its editable area is
  // .cm-content (the hidden .cm-textarea helper would also match textbox).
  await page.getByRole('dialog').locator('.cm-content').fill(
    '[{"url": "https://example.com/b1", "code": "e2e-b1"}, {"url": "https://example.com/b2", "code": "e2e-b2"}]',
  )
  // JSON syntax highlighting is active: tokens are wrapped in highlight spans.
  await expect(page.getByRole('dialog').locator('.cm-content span').first()).toBeVisible()
  await page.getByRole('button', { name: /^Create$/ }).click()
  // Result rows are the stable assertion; the success toast is transient.
  await expect(page.getByText('e2e-b1', { exact: true })).toBeVisible({ timeout: 15_000 })
  await expect(page.getByText('e2e-b2', { exact: true })).toBeVisible({ timeout: 15_000 })
})

test('imports the wrapped export dump (meta + items)', async ({ page }) => {
  await page.goto('/admin/links')
  await page.getByRole('button', { name: /import/i }).click()
  await page.getByRole('dialog').locator('.cm-content').fill(
    '{"meta": {"site": "gourl", "version": "dev", "count": 1, "exported_at": "2026/08/22 14:00"}, ' +
      '"items": [{"url": "https://example.com/dump", "code": "e2e-dump"}]}',
  )
  await page.getByRole('button', { name: /^Create$/ }).click()
  // The meta wrapper is ignored; the item inside is created.
  await expect(page.getByText('e2e-dump', { exact: true })).toBeVisible({ timeout: 15_000 })
})

test('batch create shows line numbers and flags invalid lines on blur', async ({ page }) => {
  await page.goto('/admin/links')
  await page.getByRole('button', { name: /batch create/i }).click()
  const dialog = page.getByRole('dialog')
  const editor = dialog.locator('.cm-content')
  await editor.fill(
    '[e2e-ok](2030/12/31)https://example.com/ok\n[e2e-bad](2030/13/01)https://example.com/bad\n[e2e-broken]',
  )
  // Custom line highlighting: [code] amber, (date) blue, url green.
  await expect(dialog.locator('.cm-content .tok-code').first()).toBeVisible()
  await expect(dialog.locator('.cm-content .tok-date').first()).toBeVisible()
  await expect(dialog.locator('.cm-content .tok-url').first()).toBeVisible()
  // Validation is deferred: nothing is flagged while the editor holds focus.
  await expect(dialog.locator('.cm-gutterElement.cm-error-gutter')).toHaveCount(0)
  // Line numbers 1..3 are always visible in the gutter (the hidden spacer
  // element that measures gutter width is filtered out by :visible).
  const lineNumbers = dialog.locator('.cm-lineNumbers .cm-gutterElement:visible')
  await expect(lineNumbers.nth(0)).toHaveText('1')
  await expect(lineNumbers.nth(2)).toHaveText('3')
  // Blurring the editor runs the check: lines 2 (bad date) and 3 (missing
  // url) are flagged in red.
  await dialog.getByRole('button', { name: /^Create$/ }).click()
  await expect(dialog.locator('.cm-gutterElement.cm-error-gutter')).toHaveCount(2)
  // Create refuses to submit while invalid lines remain.
  await expect(page.getByText('e2e-ok', { exact: true })).toHaveCount(0)
  // Fix the flagged lines: the flags clear on the next check and create
  // works. The last line uses a non-http scheme, which batch create accepts.
  await editor.fill(
    '[e2e-ok](2030/12/31)https://example.com/ok\n[e2e-bad]https://example.com/bad\n[e2e-ftp]ftp://example.com/file',
  )
  await dialog.getByRole('button', { name: /^Create$/ }).click()
  await expect(dialog.locator('.cm-gutterElement.cm-error-gutter')).toHaveCount(0)
  await expect(page.getByText('e2e-bad', { exact: true })).toBeVisible({ timeout: 15_000 })
  await expect(page.getByText('e2e-ftp', { exact: true })).toBeVisible({ timeout: 15_000 })
})

test('edits a link title', async ({ page, request }) => {
  await createLinkApi(request, { url: 'https://example.com/edit-me', code: 'e2e-edit' })
  await page.goto('/admin/links')
  await page.getByRole('button', { name: /edit/i }).first().click()
  await page.getByLabel('Destination URL').fill('https://example.com/changed')
  await page.getByRole('button', { name: /^Save$/ }).click()
  await expect(page.getByText('https://example.com/changed').first()).toBeVisible({ timeout: 15_000 })
})

test('exports links as markdown', async ({ page, request }) => {
  await createLinkApi(request, { url: 'https://example.com/md-target', code: 'e2e-md' })
  await page.goto('/admin/links')
  await page.getByRole('button', { name: /export/i }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('button', { name: 'Markdown' })).toBeVisible()
  const [download] = await Promise.all([
    page.waitForEvent('download'),
    dialog.getByRole('button', { name: 'Markdown' }).click(),
  ])
  // Filename shape matches exportFilename: gourl-links-<timestamp>.md
  expect(download.suggestedFilename()).toMatch(/^gourl-links-\d{4}-\d{2}-\d{2}-\d{2}-\d{2}-\d{2}\.md$/)
  const body = await new Promise<string>((resolve) => {
    download.createReadStream().then((stream) => {
      const chunks: Buffer[] = []
      stream.on('data', (c: Buffer) => chunks.push(c))
      stream.on('end', () => resolve(Buffer.concat(chunks).toString('utf-8')))
    })
  })
  // The markdown table carries the created link with its escapes intact.
  expect(body).toContain('| code | url | title | description | click_count | expires_at | created_at |')
  expect(body).toContain('`e2e-md`')
  expect(body).toContain('[https://example.com/md-target](https://example.com/md-target)')
})

test('searches and filters links', async ({ page, request }) => {
  await createLinkApi(request, { url: 'https://example.com/find-me', code: 'find-me' })
  await createLinkApi(request, { url: 'https://example.com/other', code: 'other-code' })
  await page.goto('/admin/links')
  await page.getByPlaceholder(/search/i).fill('find-me')
  await expect(page.getByText('find-me', { exact: true })).toBeVisible({ timeout: 15_000 })
  await expect(page.getByText('other-code', { exact: true })).not.toBeVisible({ timeout: 15_000 })
})

test('deletes a link', async ({ page, request }) => {
  await createLinkApi(request, { url: 'https://example.com/gone', code: 'e2e-gone' })
  await page.goto('/admin/links')
  // Row action (title="Delete"), then the confirmation inside the dialog.
  await page.getByRole('button', { name: 'Delete' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('button', { name: 'Delete' }).click()
  // The dialog closes after the delete succeeds; the row disappears.
  await expect(dialog).not.toBeVisible({ timeout: 15_000 })
  await expect(page.getByRole('row').getByText('e2e-gone', { exact: true })).not.toBeVisible({ timeout: 15_000 })
})
