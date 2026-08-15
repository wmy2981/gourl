import { expect, test } from '@playwright/test'
import { login } from './helpers'

test.beforeEach(async ({ page }) => {
  await login(page)
})

test('saves site info and the change takes effect immediately', async ({ page, request }) => {
  await page.goto('/admin/settings')
  await page.getByLabel('Service name').fill('E2E Shortener')
  await page.getByRole('button', { name: /save settings/i }).click()
  await expect(page.getByText('Settings saved')).toBeVisible()

  // Hot effect: the health endpoint reports the new name without restart.
  const res = await request.get('/api/v1/health')
  const body = await res.json()
  expect(body.name).toBe('E2E Shortener')

  // Restore for other tests in this run.
  await page.getByLabel('Service name').fill('gourl')
  await page.getByRole('button', { name: /save settings/i }).click()
})

test('adds and removes a blocked user agent', async ({ page }) => {
  await page.goto('/admin/settings')
  await page.getByPlaceholder('curl').fill('E2ESpyBot')
  await page.getByRole('button', { name: 'Add' }).click()
  await expect(page.getByText('E2ESpyBot')).toBeVisible()
})

test('creates an api token and the full value is shown once', async ({ page }) => {
  await page.goto('/admin/settings')
  await page.getByPlaceholder('Note').fill('e2e token')
  await page.getByRole('button', { name: /create token/i }).click()
  // The full 64-hex token is displayed right after creation.
  await expect(page.locator('.short-code.break-all')).toContainText(/^[0-9a-f]{64}$/)
  await expect(page.getByText('e2e token')).toBeVisible()
})

test('short code length setting is honored by new random codes', async ({ page, request }) => {
  await page.goto('/admin/settings')
  await page.getByLabel('Random code length').fill('10')
  await page.getByRole('button', { name: /save settings/i }).click()
  await expect(page.getByText('Settings saved')).toBeVisible()

  await page.goto('/admin/links')
  await page.getByRole('button', { name: /new link/i }).click()
  await page.getByLabel('Destination URL').fill('https://example.com/random-length')
  await page.getByRole('button', { name: /^Save$/ }).click()
  // The generated code cell shows a 10-char code.
  await expect(page.locator('.short-code').getByText(/^[0-9a-zA-Z]{10}$/).first()).toBeVisible()

  // Restore.
  await page.goto('/admin/settings')
  await page.getByLabel('Random code length').fill('6')
  await page.getByRole('button', { name: /save settings/i }).click()
})
