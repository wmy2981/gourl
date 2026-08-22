import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import BatchCreateDialog from '../components/BatchCreateDialog'
import { ToastProvider } from '../components/ui'
import { api } from '../lib/api'
import '../lib/i18n'

vi.mock('../lib/api', () => ({
  api: { batchCreate: vi.fn() },
  ApiError: class ApiError extends Error {},
  isApp: () => false,
}))

function renderDialog() {
  return render(
    <ToastProvider>
      <BatchCreateDialog open={true} onClose={() => {}} onCreated={() => {}} />
    </ToastProvider>,
  )
}

function editor() {
  return document.querySelector('.cm-content') as HTMLElement
}

// userEvent.keyboard parses [ and ] as key descriptors, and jsdom ships no
// DataTransfer/ClipboardEvent — hand CodeMirror a minimal clipboard (it only
// calls getData) via the paste event it handles natively.
async function paste(text: string) {
  const ev = new Event('paste', { bubbles: true, cancelable: true })
  Object.defineProperty(ev, 'clipboardData', {
    value: { getData: (type: string) => (type === 'text/plain' ? text : '') },
  })
  editor().dispatchEvent(ev)
}

// CodeMirror dispatches focus changes on a ~10ms timer; give the focus
// transaction time to land so a later blur isn't racing it.
async function settle() {
  await new Promise((r) => setTimeout(r, 30))
}

// jsdom reports navigator.language "en-US", so the UI renders English.
const createButton = () => screen.getByRole('button', { name: /^Create$/ })
const lineError = () => screen.queryByText(/Line 1/)

beforeEach(() => {
  vi.mocked(api.batchCreate).mockReset()
})

describe('BatchCreateDialog validation timing', () => {
  it('does not flag invalid lines while typing', async () => {
    const user = userEvent.setup()
    renderDialog()
    await user.click(editor())
    await paste('[bad-line]')
    await settle()
    // No error list and no gutter flags before blur/create.
    expect(lineError()).toBeNull()
    expect(document.querySelectorAll('.cm-error-gutter')).toHaveLength(0)
  })

  it('flags invalid lines when the editor loses focus', async () => {
    const user = userEvent.setup()
    renderDialog()
    await user.click(editor())
    await paste('[bad-line]')
    await user.tab() // leave the editor → blur fires
    expect(await screen.findByText(/Line 1/)).not.toBeNull()
    expect(document.querySelectorAll('.cm-error-gutter')).toHaveLength(1)
  })

  it('refuses to submit invalid lines on create click', async () => {
    vi.mocked(api.batchCreate).mockResolvedValue({
      created: 0,
      failed: 0,
      succeeded: 0,
      skipped: 0,
      updated: 0,
      results: [],
    } as never)
    const user = userEvent.setup()
    renderDialog()
    await user.click(editor())
    await paste('[bad-line]')
    await settle()
    await user.click(createButton())
    // Validation ran (error surfaced) but nothing was sent.
    expect(lineError()).not.toBeNull()
    expect(vi.mocked(api.batchCreate)).not.toHaveBeenCalled()
  })

  it('submits valid lines on create click', async () => {
    vi.mocked(api.batchCreate).mockResolvedValue({
      created: 1,
      failed: 0,
      succeeded: 1,
      skipped: 0,
      updated: 0,
      results: [{ status: 'created' }],
    } as never)
    const user = userEvent.setup()
    renderDialog()
    await user.click(editor())
    await paste('[ok](2030/12/31)https://example.com/page')
    await settle()
    await user.click(createButton())
    await waitFor(() => expect(vi.mocked(api.batchCreate)).toHaveBeenCalledTimes(1))
    expect(vi.mocked(api.batchCreate).mock.calls[0][0]).toEqual([
      { url: 'https://example.com/page', code: 'ok', expires_at: expect.any(Number) },
    ])
  })
})
