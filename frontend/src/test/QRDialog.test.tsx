import { describe, expect, it, vi } from 'vitest'
import { act, render, screen } from '@testing-library/react'
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

  it('keeps the download button in the DOM but hidden', () => {
    const link = makeLink('abc', ['a.example'])
    const { container } = render(<QRDialog link={link} urls={makeUrls(link.code, ['a.example'])} open={true} onClose={() => {}} />)
    // The JPEG download stays implemented but is hidden for now — a `hidden`
    // button drops out of the accessibility tree (its name is uncomputable),
    // so assert on the DOM node directly.
    const button = container.querySelector<HTMLButtonElement>('button[hidden]')
    expect(button).not.toBeNull()
    expect(button!.getAttribute('aria-label')).toMatch(/download qr code/i)
    expect(button).not.toBeVisible()
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

  it('keeps the QR content while the dialog animates out', () => {
    vi.useFakeTimers()
    try {
      const link = makeLink('abc', ['a.example', 'b.example'])
      const { rerender } = render(
        <QRDialog
          link={link}
          urls={makeUrls(link.code, ['a.example', 'b.example'])}
          open={true}
          onClose={() => {}}
        />,
      )
      expect(activeHost()).toBe('a.example')

      // The parent nulls the link and urls in the same render that closes —
      // the panel must keep the QR (and its height) until pop-out finishes,
      // not collapse to the error paragraph the moment close starts.
      rerender(<QRDialog link={null} urls={[]} open={false} onClose={() => {}} />)
      expect(activeHost()).toBe('a.example')
      expect(screen.getByText('abc')).toBeInTheDocument()

      // After the 180ms exit animation the dialog unmounts for good.
      act(() => {
        vi.advanceTimersByTime(200)
      })
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })
})
