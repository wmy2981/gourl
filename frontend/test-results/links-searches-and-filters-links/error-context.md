# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: links.spec.ts >> searches and filters links
- Location: e2e\links.spec.ts:55:1

# Error details

```
Test timeout of 30000ms exceeded while running "beforeEach" hook.
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
  1  | import { expect, type Page, type APIRequestContext } from '@playwright/test'
  2  | 
  3  | export const ADMIN_PASSWORD = 'e2e-password'
  4  | 
  5  | // Login through the UI.
  6  | export async function login(page: Page) {
  7  |   await page.goto('/admin/login')
> 8  |   await page.getByLabel('Password').fill(ADMIN_PASSWORD)
     |                                     ^ Error: locator.fill: Test timeout of 30000ms exceeded.
  9  |   await page.getByRole('button', { name: /sign in/i }).click()
  10 |   await expect(page).toHaveURL(/\/admin$/)
  11 | }
  12 | 
  13 | // Authenticate the API request context (its cookie jar persists).
  14 | export async function apiLogin(request: APIRequestContext) {
  15 |   const res = await request.post('/api/v1/auth/login', {
  16 |     data: { password: ADMIN_PASSWORD },
  17 |   })
  18 |   expect(res.status()).toBe(200)
  19 | }
  20 | 
  21 | export interface Link {
  22 |   code: string
  23 |   url: string
  24 |   title: string
  25 |   description: string
  26 |   expires_at: number
  27 |   click_count: number
  28 |   created_at: number
  29 |   updated_at: number
  30 |   urls: string[]
  31 | }
  32 | 
  33 | export async function createLinkApi(
  34 |   request: APIRequestContext,
  35 |   body: { url: string; code?: string; expires_at?: number },
  36 | ): Promise<Link> {
  37 |   const res = await request.post('/api/v1/links', { data: body })
  38 |   expect(res.status()).toBe(201)
  39 |   return (await res.json()) as Link
  40 | }
  41 | 
  42 | // Create a link through the UI form.
  43 | export async function createLinkUi(page: Page, url: string, code?: string) {
  44 |   await page.getByRole('button', { name: /new link/i }).click()
  45 |   await page.getByLabel('Destination URL').fill(url)
  46 |   if (code) {
  47 |     await page.getByLabel(/Short code/).fill(code)
  48 |   }
  49 |   await page.getByRole('button', { name: /^Save$/ }).click()
  50 | }
  51 | 
  52 | export async function waitForToast(page: Page, text: string) {
  53 |   await expect(page.getByText(text).first()).toBeVisible()
  54 | }
  55 | 
```