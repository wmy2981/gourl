import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import Connect from '../pages/Connect'
import { ToastProvider } from '../components/ui'
import '../lib/i18n'

function renderConnect() {
  return render(
    <ToastProvider>
      <MemoryRouter initialEntries={['/admin/connect']}>
        <Routes>
          <Route path="/admin/connect" element={<Connect />} />
          <Route path="/admin" element={<div>admin-dashboard</div>} />
        </Routes>
      </MemoryRouter>
    </ToastProvider>,
  )
}

// The connect flow probes GET /api/v1/auth/status before persisting.
function mockAuthOk() {
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async () =>
        new Response(JSON.stringify({ configured: true }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
    ),
  )
}

async function fillAndSubmit(url: string) {
  const user = userEvent.setup()
  await user.type(screen.getByLabelText('Server address'), url)
  await user.type(screen.getByLabelText('API token'), 'tok-123')
  await user.click(screen.getByRole('button', { name: 'Connect' }))
  return user
}

beforeEach(() => {
  localStorage.clear()
  vi.unstubAllGlobals()
})

describe('Connect', () => {
  it('connects to an https server without a confirm dialog', async () => {
    mockAuthOk()
    renderConnect()
    await fillAndSubmit('https://gourl.example.com')

    expect(screen.queryByText('Insecure connection')).not.toBeInTheDocument()
    expect(await screen.findByText('admin-dashboard')).toBeInTheDocument()
    expect(localStorage.getItem('gourl-server')).toContain('https://gourl.example.com')
  })

  it('asks before connecting to a plain-http server and cancels on decline', async () => {
    mockAuthOk()
    renderConnect()
    const user = await fillAndSubmit('http://192.168.1.10:8081')

    expect(await screen.findByText('Insecure connection')).toBeInTheDocument()
    // The dialog names the server; the input field shows the same URL, so
    // scope the assertion to the dialog.
    expect(screen.getByRole('dialog')).toHaveTextContent(/192\.168\.1\.10:8081/)

    await user.click(screen.getByRole('button', { name: /cancel/i }))
    expect(localStorage.getItem('gourl-server')).toBeNull()
    expect(fetch).not.toHaveBeenCalled()
    expect(screen.queryByText('admin-dashboard')).not.toBeInTheDocument()
  })

  it('remembers the origin after confirming', async () => {
    mockAuthOk()
    renderConnect()
    const user = await fillAndSubmit('http://192.168.1.10:8081')

    await user.click(await screen.findByRole('button', { name: /trust & connect/i }))
    expect(await screen.findByText('admin-dashboard')).toBeInTheDocument()

    const trusted = JSON.parse(
      localStorage.getItem('gourl-trusted-insecure-hosts') ?? '[]',
    ) as string[]
    expect(trusted).toContain('http://192.168.1.10:8081')
  })

  it('skips the dialog for a previously trusted origin', async () => {
    localStorage.setItem(
      'gourl-trusted-insecure-hosts',
      JSON.stringify(['http://192.168.1.10:8081']),
    )
    mockAuthOk()
    renderConnect()
    await fillAndSubmit('http://192.168.1.10:8081')

    expect(screen.queryByText('Insecure connection')).not.toBeInTheDocument()
    expect(await screen.findByText('admin-dashboard')).toBeInTheDocument()
  })
})
