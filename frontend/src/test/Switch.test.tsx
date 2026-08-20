import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Switch } from '../components/ui'

// Every click goes through the app-mode haptic path; the no-bridge fallback
// (plain web) is what the first test exercises.
vi.mock('../lib/api', () => ({ isApp: () => true }))

describe('Switch', () => {
  afterEach(() => {
    delete (window as unknown as Record<string, unknown>).GourlBridge
  })

  it('toggles aria-checked and reports the new value', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    const { rerender } = render(<Switch checked={false} onChange={onChange} aria-label="noTextSelect" />)
    const sw = screen.getByRole('switch', { name: 'noTextSelect' })
    expect(sw).toHaveAttribute('aria-checked', 'false')

    await user.click(sw)
    expect(onChange).toHaveBeenLastCalledWith(true)

    rerender(<Switch checked={true} onChange={onChange} aria-label="noTextSelect" />)
    expect(sw).toHaveAttribute('aria-checked', 'true')
  })

  it('fires the native toggle haptic with the new state', async () => {
    const user = userEvent.setup()
    const switchHaptic = vi.fn()
    ;(window as unknown as Record<string, unknown>).GourlBridge = { switchHaptic }

    const { rerender } = render(<Switch checked={false} onChange={() => {}} aria-label="noTextSelect" />)
    const sw = screen.getByRole('switch', { name: 'noTextSelect' })

    await user.click(sw)
    expect(switchHaptic).toHaveBeenLastCalledWith(true)

    rerender(<Switch checked={true} onChange={() => {}} aria-label="noTextSelect" />)
    await user.click(sw)
    expect(switchHaptic).toHaveBeenLastCalledWith(false)
  })
})
