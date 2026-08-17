import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import QRDialog from '../components/QRDialog'
import type { Link } from '../lib/api'
import '../lib/i18n'

function makeLink(code: string, hosts: string[]): Link {
  return {
    id: 1,
    code,
    url: `https://${hosts[0]}/${code}`,
    title: '',
    description: '',
    expires_at: 0,
    click_count: 0,
    created_at: 0,
    updated_at: 0,
  }
}

function makeUrls(code: string, hosts: string[]): string[] {
  return hosts.map((h) => `https://${h}/${code}`)
}

function activeHost() {
  const buttons = Array.from(screen.getAllByRole('button'))
  const picked = buttons.find((b) => b.className.includes('bg-accent-soft'))
  return picked ? picked.textContent : null
}

describe('QRDialog', () => {
  it('opens on the base URL picked on the row, not the first one', () => {
    const link = makeLink('abc', ['a.example', 'b.example'])
    const urls = makeUrls(link.code, ['a.example', 'b.example'])
    render(<QRDialog link={link} urls={urls} open={true} onClose={() => {}} initialIndex={1} />)

    // First-open regression: previously the still-null `shown` clamped the
    // index to 0, so the QR always rendered the first base URL.
    expect(activeHost()).toBe('b.example')
    expect(activeHost()).not.toBe('a.example')
  })

  it('switches the QR variant when a variant button is clicked', async () => {
    const user = userEvent.setup()
    const link = makeLink('abc', ['a.example', 'b.example'])
    render(<QRDialog link={link} urls={makeUrls(link.code, ['a.example', 'b.example'])} open={true} onClose={() => {}} />)

    expect(activeHost()).toBe('a.example')
    await user.click(screen.getByRole('button', { name: 'b.example' }))
    expect(activeHost()).toBe('b.example')
  })

  it('renders a download button next to the close button', () => {
    const link = makeLink('abc', ['a.example'])
    render(<QRDialog link={link} urls={makeUrls(link.code, ['a.example'])} open={true} onClose={() => {}} />)
    expect(screen.getByRole('button', { name: /download qr code/i })).toBeInTheDocument()
  })

  it('re-clamps the index against the new link when reopened', () => {
    const first = makeLink('abc', ['a.example', 'b.example'])
    const { rerender } = render(
      <QRDialog
        link={first}
        urls={makeUrls(first.code, ['a.example', 'b.example'])}
        open={true}
        onClose={() => {}}
        initialIndex={1}
      />,
    )
    expect(activeHost()).toBe('b.example')

    // Reopen on a different link — the index must reset against the new
    // link's URL set, not the previous one's.
    const second = makeLink('def', ['c.example', 'd.example'])
    rerender(
      <QRDialog
        link={second}
        urls={makeUrls(second.code, ['c.example', 'd.example'])}
        open={true}
        onClose={() => {}}
        initialIndex={1}
      />,
    )
    expect(activeHost()).toBe('d.example')
  })
})
