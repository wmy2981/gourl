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
  await page.getByRole('dialog').getByRole('textbox').fill(
    '[{"url": "https://example.com/b1", "code": "e2e-b1"}, {"url": "https://example.com/b2", "code": "e2e-b2"}]',
  )
  await page.getByRole('button', { name: /^Create$/ }).click()
  // Result rows are the stable assertion; the success toast is transient.
  await expect(page.getByText('e2e-b1', { exact: true })).toBeVisible({ timeout: 15_000 })
  await expect(page.getByText('e2e-b2', { exact: true })).toBeVisible({ timeout: 15_000 })
})

test('edits a link title', async ({ page, request }) => {
  await createLinkApi(request, { url: 'https://example.com/edit-me', code: 'e2e-edit' })
  await page.goto('/admin/links')
  await page.getByRole('button', { name: /edit/i }).first().click()
  await page.getByLabel('Destination URL').fill('https://example.com/changed')
  await page.getByRole('button', { name: /^Save$/ }).click()
  await expect(page.getByText('https://example.com/changed').first()).toBeVisible({ timeout: 15_000 })
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
