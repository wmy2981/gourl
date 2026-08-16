import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import Login from '../pages/Login'
import { ToastProvider } from '../components/ui'
import '../lib/i18n'

function renderLogin() {
  return render(
    <MemoryRouter>
      <ToastProvider>
        <Login />
      </ToastProvider>
    </MemoryRouter>,
  )
}

function mockFetch(status: number, body: unknown) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async () =>
      new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
      }),
    ),
  )
}

beforeEach(() => {
  vi.unstubAllGlobals()
})

describe('Login', () => {
  it('submits the password and navigates on success', async () => {
    mockFetch(200, { ok: true })
    renderLogin()

    await userEvent.type(screen.getByLabelText('Password'), 'secret')
    await userEvent.click(screen.getByRole('button', { name: /sign in/i }))

    expect(fetch).toHaveBeenCalledTimes(1)
    const [, init] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0]
    expect((init as RequestInit).body).toContain('secret')
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
