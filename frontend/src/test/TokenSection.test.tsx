import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import '../lib/i18n'
import { TokenSection } from '../pages/Settings'
import type { TokenInfo } from '../lib/api'

vi.mock('../lib/api', () => {
  return {
    ApiError: class ApiError extends Error {},
    isApp: () => false,
    getServerConfig: () => null,
    api: {
      tokens: vi.fn(),
      deleteToken: vi.fn(),
    },
  }
})

import { api } from '../lib/api'

const tok: TokenInfo = { id: 1, token: 'abc12345', note: 'ci token', created_at: 0 }

const renderSection = () => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <TokenSection
          tokenNote=""
          setTokenNote={() => {}}
          newToken=""
          create={() => {}}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('TokenSection', () => {
  // The API only returns the 8-char prefix; the list must present it as a
  // truncated preview (ellipsis), not as if it were the full secret.
  it('renders the prefix with an ellipsis and the note in the list', async () => {
    vi.mocked(api.tokens).mockResolvedValue({ tokens: [tok] })
    renderSection()
    expect(await screen.findByText('abc12345…')).toBeInTheDocument()
    expect(screen.getByText('ci token')).toBeInTheDocument()
  })

  it('shows the prefixed token plus its note in the delete dialog', async () => {
    const user = userEvent.setup()
    vi.mocked(api.tokens).mockResolvedValue({ tokens: [tok] })
    renderSection()
    await user.click(await screen.findByRole('button', { name: 'Delete token' }))
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveTextContent('abc12345…')
    expect(dialog).toHaveTextContent('ci token')
  })
})
