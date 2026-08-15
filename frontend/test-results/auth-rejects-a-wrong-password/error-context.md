# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: auth.spec.ts >> rejects a wrong password
- Location: e2e\auth.spec.ts:3:1

# Error details

```
Test timeout of 30000ms exceeded.
```

```
Error: locator.fill: Test timeout of 30000ms exceeded.
Call log:
  - waiting for getByLabel('Password')

```

# Page snapshot

```yaml
- main [ref=e2]:
  - heading "Page not found" [level=1] [ref=e3]
  - paragraph [ref=e4]: The short link you visited does not exist or has been removed.
```

# Test source

```ts
  1  | import { expect, test } from '@playwright/test'
  2  | 
  3  | test('rejects a wrong password', async ({ page }) => {
  4  |   await page.goto('/admin/login')
> 5  |   await page.getByLabel('Password').fill('wrong-password')
     |                                     ^ Error: locator.fill: Test timeout of 30000ms exceeded.
  6  |   await page.getByRole('button', { name: /sign in/i }).click()
  7  |   await expect(page.getByText('Invalid password')).toBeVisible()
  8  | })
  9  | 
  10 | test('signs in and lands on the dashboard', async ({ page }) => {
  11 |   await page.goto('/admin/login')
  12 |   await page.getByLabel('Password').fill('e2e-password')
  13 |   await page.getByRole('button', { name: /sign in/i }).click()
  14 |   await expect(page).toHaveURL(/\/admin$/)
  15 |   await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible()
  16 | })
  17 | 
  18 | test('unauthenticated visitors are redirected to login', async ({ page }) => {
  19 |   await page.goto('/admin')
  20 |   await expect(page).toHaveURL(/\/admin\/login/)
  21 | })
  22 | 
  23 | test('health endpoint is public and reports identity', async ({ request }) => {
  24 |   const res = await request.get('/api/v1/health')
  25 |   expect(res.status()).toBe(200)
  26 |   const body = await res.json()
  27 |   expect(body.name).toBe('gourl')
  28 |   expect(body.version).toMatch(/^\d+\.\d+\.\d+$/)
  29 |   expect(body.redis).toBe('ok')
  30 |   expect(body.sqlite).toBe('ok')
  31 | })
  32 | 
```