# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: redirect.spec.ts >> redirects multi-level codes
- Location: e2e\redirect.spec.ts:15:1

# Error details

```
Error: expect(received).toBe(expected) // Object.is equality

Expected: 302
Received: 404
```

# Test source

```ts
  1  | import { expect, test } from '@playwright/test'
  2  | import { apiLogin, createLinkApi } from './helpers'
  3  | 
  4  | test.beforeEach(async ({ request }) => {
  5  |   await apiLogin(request)
  6  | })
  7  | 
  8  | test('redirects a short code with 302 to the target', async ({ request }) => {
  9  |   await createLinkApi(request, { url: 'https://example.com/redirect-target', code: 'r302' })
  10 |   const res = await request.get('/r302')
  11 |   expect(res.status()).toBe(302)
  12 |   expect(res.headers()['location']).toBe('https://example.com/redirect-target')
  13 | })
  14 | 
  15 | test('redirects multi-level codes', async ({ request }) => {
  16 |   await createLinkApi(request, { url: 'https://example.com/multi', code: 'a/b' })
  17 |   const res = await request.get('/a/b')
> 18 |   expect(res.status()).toBe(302)
     |                        ^ Error: expect(received).toBe(expected) // Object.is equality
  19 |   expect(res.headers()['location']).toBe('https://example.com/multi')
  20 | })
  21 | 
  22 | test('blocks matching user agents with 403 and does not count them', async ({ request }) => {
  23 |   await createLinkApi(request, { url: 'https://example.com/ua', code: 'ua-test' })
  24 |   await request.post('/api/v1/ua-blocks', { data: { pattern: 'E2EBlockedBot' } })
  25 | 
  26 |   const blocked = await request.get('/ua-test', {
  27 |     headers: { 'User-Agent': 'Mozilla/5.0 (compatible; E2EBlockedBot/1.0)' },
  28 |   })
  29 |   expect(blocked.status()).toBe(403)
  30 | 
  31 |   // A normal UA redirects and is counted (flush runs every 500ms in e2e).
  32 |   const ok = await request.get('/ua-test', {
  33 |     headers: { 'User-Agent': 'Mozilla/5.0 Chrome/126' },
  34 |   })
  35 |   expect(ok.status()).toBe(302)
  36 | 
  37 |   const stats = await request.get('/api/v1/links/ua-test')
  38 |   expect(stats.status()).toBe(200)
  39 |   const link = await stats.json()
  40 |   expect(link.click_count).toBe(1)
  41 | })
  42 | 
  43 | test('shows an expired page with bilingual copy', async ({ request, page }) => {
  44 |   await createLinkApi(request, {
  45 |     url: 'https://example.com/expired-target',
  46 |     code: 'expired',
  47 |     expires_at: 1, // long past
  48 |   })
  49 | 
  50 |   const pageEn = await page.request.get('/expired')
  51 |   expect(pageEn.status()).toBe(200)
  52 |   expect((await pageEn.text()).toLowerCase()).toContain('this link has expired')
  53 | 
  54 |   const pageZh = await page.request.get('/expired?lang=zh')
  55 |   expect(pageZh.status()).toBe(200)
  56 |   expect((await pageZh.text())).toContain('链接已过期')
  57 | })
  58 | 
  59 | test('serves a 404 page for unknown codes and reserved prefixes', async ({ request }) => {
  60 |   expect((await request.get('/definitely-not-a-code')).status()).toBe(404)
  61 |   expect((await request.get('/expired/anything')).status()).toBe(404)
  62 | })
  63 | 
```