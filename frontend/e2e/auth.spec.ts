import { expect, test } from '@playwright/test'

test('rejects a wrong password', async ({ page }) => {
  await page.goto('/admin/login')
  await page.getByLabel('Password').fill('wrong-password')
  await page.getByRole('button', { name: /sign in/i }).click()
  await expect(page.getByText('Invalid password')).toBeVisible()
})

test('signs in and lands on the dashboard', async ({ page }) => {
  await page.goto('/admin/login')
  await page.getByLabel('Password').fill('e2e-password')
  await page.getByRole('button', { name: /sign in/i }).click()
  await expect(page).toHaveURL(/\/admin$/)
  await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible()
})

test('unauthenticated visitors are redirected to login', async ({ page }) => {
  await page.goto('/admin')
  await expect(page).toHaveURL(/\/admin\/login/)
})

test('health endpoint is public and reports identity', async ({ request }) => {
  const res = await request.get('/api/v1/health')
  expect(res.status()).toBe(200)
  const body = await res.json()
  expect(body.name).toBe('gourl')
  expect(body.version).toMatch(/^\d+\.\d+\.\d+$/)
  expect(body.redis).toBe('ok')
  expect(body.sqlite).toBe('ok')
})
