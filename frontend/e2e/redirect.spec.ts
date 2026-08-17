import { expect, test } from '@playwright/test'
import { apiLogin, createLinkApi } from './helpers'

test.beforeEach(async ({ request }) => {
  await apiLogin(request)
})

// maxRedirects: 0 — the request fixture follows redirects by default, which
// would chase the 302 to the external target and report its status instead.
const NO_REDIRECT = { maxRedirects: 0 } as const

test('redirects a short code with 302 to the target', async ({ request }) => {
  await createLinkApi(request, { url: 'https://example.com/redirect-target', code: 'r302' })
  const res = await request.get('/r302', NO_REDIRECT)
  expect(res.status()).toBe(302)
  expect(res.headers()['location']).toBe('https://example.com/redirect-target')
})

test('redirects multi-level codes', async ({ request }) => {
  await createLinkApi(request, { url: 'https://example.com/multi', code: 'a/b' })
  const res = await request.get('/a/b', NO_REDIRECT)
  expect(res.status()).toBe(302)
  expect(res.headers()['location']).toBe('https://example.com/multi')
})

test('blocks matching user agents with 403 and does not count them', async ({ request }) => {
  await createLinkApi(request, { url: 'https://example.com/ua', code: 'ua-test' })
  await request.post('/api/v1/ua-blocks', { data: { pattern: 'E2EBlockedBot' } })

  const blocked = await request.get('/ua-test', {
    headers: { 'User-Agent': 'Mozilla/5.0 (compatible; E2EBlockedBot/1.0)' },
  })
  expect(blocked.status()).toBe(403)

  // A normal UA redirects and is counted (flush runs every 500ms in e2e).
  const ok = await request.get('/ua-test', {
    headers: { 'User-Agent': 'Mozilla/5.0 Chrome/126' },
    ...NO_REDIRECT,
  })
  expect(ok.status()).toBe(302)

  // The click lands in Redis first; wait for the 500ms flush to SQLite.
  await expect
    .poll(async () => {
      const stats = await request.get('/api/v1/links/ua-test')
      return stats.status() === 200 ? (await stats.json()).click_count : -1
    }, { timeout: 10_000 })
    .toBe(1)
})

test('expired codes behave like missing ones (404, bilingual copy)', async ({ request, page }) => {
  await createLinkApi(request, {
    url: 'https://example.com/expired-target',
    code: 'expire-me',
    expires_at: 1, // long past
  })

  const pageEn = await page.request.get('/expire-me')
  expect(pageEn.status()).toBe(404)
  expect((await pageEn.text()).toLowerCase()).toContain('page not found')

  const pageZh = await page.request.get('/expire-me?lang=zh')
  expect(pageZh.status()).toBe(404)
  expect((await pageZh.text())).toContain('页面不存在')
})

test('serves a 404 page for unknown codes and reserved prefixes', async ({ request }) => {
  expect((await request.get('/definitely-not-a-code')).status()).toBe(404)
  expect((await request.get('/expired/anything')).status()).toBe(404)
})
