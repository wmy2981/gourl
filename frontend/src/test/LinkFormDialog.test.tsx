import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import LinkFormDialog from '../components/LinkFormDialog'
import { ToastProvider } from '../components/ui'
import '../lib/i18n'

function urlInput() {
  return screen.getByLabelText('Destination URL') as HTMLInputElement
}

describe('LinkFormDialog scheme buttons', () => {
  it('fills an empty input with the clicked scheme', async () => {
    const user = userEvent.setup()
    render(
      <ToastProvider>
        <LinkFormDialog link={null} open={true} onClose={() => {}} onSaved={() => {}} />
      </ToastProvider>,
    )
    await user.click(screen.getByRole('button', { name: 'https://' }))
    expect(urlInput().value).toBe('https://')
  })

  it('prepends the scheme to a scheme-less URL', async () => {
    const user = userEvent.setup()
    render(
      <ToastProvider>
        <LinkFormDialog link={null} open={true} onClose={() => {}} onSaved={() => {}} />
      </ToastProvider>,
    )
    await user.type(urlInput(), 'example.com/path')
    await user.click(screen.getByRole('button', { name: 'http://' }))
    expect(urlInput().value).toBe('http://example.com/path')
  })

  it('swaps an existing other scheme', async () => {
    const user = userEvent.setup()
    render(
      <ToastProvider>
        <LinkFormDialog link={null} open={true} onClose={() => {}} onSaved={() => {}} />
      </ToastProvider>,
    )
    await user.type(urlInput(), 'http://example.com/x')
    await user.click(screen.getByRole('button', { name: 'https://' }))
    expect(urlInput().value).toBe('https://example.com/x')
  })

  it('leaves the same scheme alone', async () => {
    const user = userEvent.setup()
    render(
      <ToastProvider>
        <LinkFormDialog link={null} open={true} onClose={() => {}} onSaved={() => {}} />
      </ToastProvider>,
    )
    await user.type(urlInput(), 'https://example.com/x')
    await user.click(screen.getByRole('button', { name: 'https://' }))
    expect(urlInput().value).toBe('https://example.com/x')
  })

  it('is case-insensitive about the existing scheme', async () => {
    const user = userEvent.setup()
    render(
      <ToastProvider>
        <LinkFormDialog link={null} open={true} onClose={() => {}} onSaved={() => {}} />
      </ToastProvider>,
    )
    await user.type(urlInput(), 'HTTP://example.com/x')
    await user.click(screen.getByRole('button', { name: 'https://' }))
    expect(urlInput().value).toBe('https://example.com/x')
  })
})
