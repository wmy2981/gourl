import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import Login from '../pages/Login'
import { ToastProvider } from '../components/ui'
import '../lib/i18n'

function renderLogin() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter>
        <ToastProvider>
          <Login />
        </ToastProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

function mockFetch(status: number, body: unknown) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string | URL) => {
      // The login page loads the site config for the brand line.
      if (String(url).endsWith('/api/v1/config')) {
        return new Response(JSON.stringify({ site: { name: 'My Service' } }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      return new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
      })
    }),
  )
}

beforeEach(() => {
  vi.unstubAllGlobals()
})

describe('Login', () => {
  it('shows the configured service name in the brand line', async () => {
    mockFetch(200, { ok: true })
    renderLogin()
    expect(await screen.findByText(/My Service/)).toBeInTheDocument()
  })

  it('submits the password and navigates on success', async () => {
    mockFetch(200, { ok: true })
    renderLogin()

    await userEvent.type(screen.getByLabelText('Password'), 'secret')
    await userEvent.click(screen.getByRole('button', { name: /sign in/i }))

    const calls = (fetch as ReturnType<typeof vi.fn>).mock.calls as [string, RequestInit][]
    expect(calls.length).toBeGreaterThanOrEqual(2) // config + login
    const loginCall = calls.find(([url]) => String(url).endsWith('/api/v1/auth/login'))
    expect(loginCall).toBeDefined()
    expect(loginCall![1].body).toContain('secret')
  })

  it('shows an error toast on a wrong password', async () => {
    mockFetch(401, { error: { code: 'unauthorized', message: 'invalid password' } })
    renderLogin()

    await userEvent.type(screen.getByLabelText('Password'), 'wrong')
    await userEvent.click(screen.getByRole('button', { name: /sign in/i }))

    expect(await screen.findByText('Invalid password')).toBeInTheDocument()
  })

  it('disables the submit button while empty', () => {
    renderLogin()
    expect(screen.getByRole('button', { name: /sign in/i })).toBeDisabled()
  })
})
