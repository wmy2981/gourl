import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import '../lib/i18n'
import { ConnectionCard } from '../pages/Settings'

// The card only exists in app mode; every probe runs through the mocked api.
vi.mock('../lib/api', () => {
  class ApiError extends Error {
    status: number
    code: string
    constructor(status: number, code: string, message: string) {
      super(message)
      this.status = status
      this.code = code
    }
  }
  return {
    ApiError,
    isApp: () => true,
    getServerConfig: () => ({ url: 'http://server:8080', token: 't' }),
    setServerConfig: vi.fn(),
    api: {
      authStatus: vi.fn(),
      getConfig: vi.fn(),
      health: vi.fn(),
    },
  }
})

import { ApiError, api } from '../lib/api'

const probeOk = () => {
  vi.mocked(api.authStatus).mockResolvedValue({
    configured: true,
    authenticated: true,
    actor: 'token',
  } as never)
  vi.mocked(api.health).mockResolvedValue({ name: 'gourl', version: '9.9.9' })
}

// The card navigates away on disconnect, so it needs a router context.
const renderCard = () => render(<MemoryRouter><ConnectionCard /></MemoryRouter>)

describe('ConnectionCard', () => {
  beforeEach(() => {
    vi.mocked(api.authStatus).mockReset()
    vi.mocked(api.getConfig).mockReset()
    vi.mocked(api.health).mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
    localStorage.removeItem('gourl-haptics')
  })

  it('shows connected, both versions and the refresh button after a successful probe', async () => {
    probeOk()
    renderCard()
    await waitFor(() => expect(screen.getByText('Connected')).toBeInTheDocument())
    // Server version comes from the health probe; the app version is the
    // embedded build constant — assert against it, never a literal, so a
    // VERSION bump doesn't break this test.
    expect(screen.getByText('v9.9.9')).toBeInTheDocument()
    expect(screen.getByText(`v${__APP_VERSION__}`)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeInTheDocument()
  })

  it('marks the token as rejected when the server answers but rejects it', async () => {
    // The new auth/status probe reports a dead token through
    // authenticated:false — a 200, not a 401.
    vi.mocked(api.authStatus).mockResolvedValue({
      configured: true,
      authenticated: false,
      actor: '',
    } as never)
    vi.mocked(api.health).mockResolvedValue({ name: 'gourl', version: '9.9.9' })
    renderCard()
    await waitFor(() => expect(screen.getByText('Token rejected')).toBeInTheDocument())
  })

  it('marks the token as rejected on a 401 from the status probe', async () => {
    vi.mocked(api.authStatus).mockRejectedValue(new ApiError(401, 'unauthorized', 'token invalid'))
    vi.mocked(api.health).mockResolvedValue({ name: 'gourl', version: '9.9.9' })
    renderCard()
    await waitFor(() => expect(screen.getByText('Token rejected')).toBeInTheDocument())
  })

  it('falls back to the config probe against a legacy server without the authenticated field', async () => {
    // Pre-auth/status servers omit authenticated; the card re-judges via
    // GET /config's 401-based logic.
    vi.mocked(api.authStatus).mockResolvedValue({ configured: true } as never)
    vi.mocked(api.getConfig).mockResolvedValue({} as never)
    vi.mocked(api.health).mockResolvedValue({ name: 'gourl', version: '9.9.9' })
    renderCard()
    await waitFor(() => expect(screen.getByText('Connected')).toBeInTheDocument())
    expect(vi.mocked(api.getConfig)).toHaveBeenCalled()

    // And a 401 there still means the token is dead.
    vi.mocked(api.authStatus).mockReset()
    vi.mocked(api.getConfig).mockReset()
    vi.mocked(api.authStatus).mockResolvedValue({ configured: true } as never)
    vi.mocked(api.getConfig).mockRejectedValue(new ApiError(401, 'unauthorized', 'token invalid'))
    await act(async () => {
      await userEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    })
    await waitFor(() => expect(screen.getByText('Token rejected')).toBeInTheDocument())
  })

  it('marks the server unreachable on a network error', async () => {
    vi.mocked(api.authStatus).mockRejectedValue(new TypeError('fetch failed'))
    vi.mocked(api.health).mockResolvedValue({ name: 'gourl', version: '9.9.9' })
    renderCard()
    await waitFor(() => expect(screen.getByText('Cannot reach the server')).toBeInTheDocument())
  })

  it('shows a dash for the server version when health is unavailable', async () => {
    probeOk()
    vi.mocked(api.health).mockRejectedValue(new Error('boom'))
    renderCard()
    await waitFor(() => expect(screen.getByText('Connected')).toBeInTheDocument())
    // The address line shows the real url, so the only "—" is the version.
    expect(screen.getByText('—')).toBeInTheDocument()
  })

  it('flags the connection unreachable when a probe gets no answer within 5s', async () => {
    vi.useFakeTimers()
    vi.mocked(api.health).mockResolvedValue({ name: 'gourl', version: '9.9.9' })
    // A probe that never settles: only the abort signal can end it.
    vi.mocked(api.authStatus).mockImplementation(
      (init?: RequestInit) =>
        new Promise((_, reject) => {
          init?.signal?.addEventListener('abort', () => reject(init.signal?.reason ?? new Error('aborted')))
        }),
    )
    renderCard()
    expect(screen.getByText('Checking…')).toBeInTheDocument()
    // The abort fires at 5s; waitFor would hang under fake timers, so the
    // act() flush is the synchronization point.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_000)
    })
    expect(screen.getByText('Cannot reach the server')).toBeInTheDocument()
  })

  it('re-probes immediately when the refresh button is clicked', async () => {
    probeOk()
    const user = userEvent.setup()
    renderCard()
    await waitFor(() => expect(screen.getByText('Connected')).toBeInTheDocument())
    const before = vi.mocked(api.authStatus).mock.calls.length
    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    await waitFor(() => expect(vi.mocked(api.authStatus).mock.calls.length).toBe(before + 1))
    // The probe feedback loop settles back on connected.
    await waitFor(() => expect(screen.getByText('Connected')).toBeInTheDocument())
  })

  it('re-probes on the 10s polling interval', async () => {
    vi.useFakeTimers()
    probeOk()
    renderCard()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(screen.getByText('Connected')).toBeInTheDocument()
    const afterMount = vi.mocked(api.authStatus).mock.calls.length
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000)
    })
    expect(vi.mocked(api.authStatus).mock.calls.length).toBe(afterMount + 1)
  })

  it('renders the haptics toggle defaulting to on', () => {
    probeOk()
    renderCard()
    expect(screen.getByRole('switch', { name: 'Vibration feedback' })).toBeChecked()
  })

  it('toggles the haptics setting off and back on', async () => {
    probeOk()
    renderCard()
    const user = userEvent.setup()
    const toggle = screen.getByRole('switch', { name: 'Vibration feedback' })
    await user.click(toggle)
    expect(localStorage.getItem('gourl-haptics')).toBe('0')
    await user.click(toggle)
    expect(localStorage.getItem('gourl-haptics')).toBeNull()
  })
})
