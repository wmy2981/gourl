import { expect, type Page, type APIRequestContext } from '@playwright/test'

export const ADMIN_PASSWORD = 'e2e-password'

// Login through the UI.
export async function login(page: Page) {
  await page.goto('/admin/login')
  await page.getByLabel('Password').fill(ADMIN_PASSWORD)
  await page.getByRole('button', { name: /sign in/i }).click()
  await expect(page).toHaveURL(/\/admin$/)
}

// Authenticate the API request context (its cookie jar persists).
export async function apiLogin(request: APIRequestContext) {
  const res = await request.post('/api/v1/auth/login', {
    data: { password: ADMIN_PASSWORD },
  })
  expect(res.status()).toBe(200)
}

export interface Link {
  code: string
  url: string
  title: string
  description: string
  expires_at: number
  click_count: number
  created_at: number
  updated_at: number
  urls: string[]
}

export async function createLinkApi(
  request: APIRequestContext,
  body: { url: string; code?: string; expires_at?: number },
): Promise<Link> {
  const res = await request.post('/api/v1/links', { data: body })
  expect(res.status()).toBe(201)
  return (await res.json()) as Link
}

// Create a link through the UI form (navigates to the links page first).
export async function createLinkUi(page: Page, url: string, code?: string) {
  await page.goto('/admin/links')
  await page.getByRole('button', { name: /new link/i }).click()
  await page.getByLabel('Destination URL').fill(url)
  if (code) {
    await page.getByLabel(/Short code/).fill(code)
  }
  await page.getByRole('button', { name: /^Save$/ }).click()
}

export async function waitForToast(page: Page, text: string) {
  await expect(page.getByText(text).first()).toBeVisible()
}
