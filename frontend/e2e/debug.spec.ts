import { expect, test } from '@playwright/test'

test('debug 404', async ({ request }) => {
  const login = await request.post('/api/v1/auth/login', { data: { password: 'e2e-password' } })
  console.log('login status:', login.status())
  const created = await request.post('/api/v1/links', {
    data: { url: 'https://example.com/target', code: 'r302' },
  })
  console.log('create status:', created.status(), await created.text())
  const res = await request.get('/r302')
  console.log('GET /r302 status:', res.status())
  console.log('GET /r302 body:', (await res.text()).slice(0, 300))
  const res2 = await request.get('/api/v1/links/r302')
  console.log('GET link status:', res2.status(), (await res2.text()).slice(0, 300))
})
