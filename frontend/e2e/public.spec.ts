import { expect, test } from '@playwright/test'

// maxRedirects: 0 — the public page must answer 200 directly at /, never
// bounce to /admin like the old root redirect.
const NO_REDIRECT = { maxRedirects: 0 } as const

test('serves the public landing page at / with the config name, icon and bilingual notice', async ({ request }) => {
  const en = await request.get('/', NO_REDIRECT)
  expect(en.status()).toBe(200)
  const enBody = await en.text()
  expect(enBody).toContain('<h1>gourl</h1>')
  expect(enBody).toContain('This is a short link service')
  expect(enBody).toContain('/favicon.svg')

  const zh = await request.get('/?lang=zh', NO_REDIRECT)
  expect(zh.status()).toBe(200)
  expect((await zh.text())).toContain('短链接服务')
})

test('the icon endpoint serves the built-in icon when no custom icon is uploaded', async ({ request }) => {
  const res = await request.get('/favicon.svg')
  expect(res.status()).toBe(200)
  expect(res.headers()['content-type']).toContain('image/svg+xml')
})
