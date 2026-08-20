import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Button } from '../components/ui'

// Every click goes through the app-mode haptic path; the no-bridge fallback
// (plain web) is what the second test exercises.
vi.mock('../lib/api', () => ({ isApp: () => true }))

describe('Button', () => {
  afterEach(() => {
    delete (window as unknown as Record<string, unknown>).GourlBridge
  })

  it('fires the button haptic and still calls onClick in app mode', async () => {
    const user = userEvent.setup()
    const buttonHaptic = vi.fn()
    ;(window as unknown as Record<string, unknown>).GourlBridge = { buttonHaptic }
    const onClick = vi.fn()

    render(<Button onClick={onClick}>Go</Button>)
    await user.click(screen.getByRole('button', { name: 'Go' }))

    expect(buttonHaptic).toHaveBeenCalledOnce()
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('keeps working without a bridge (web)', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()

    render(<Button onClick={onClick}>Go</Button>)
    await user.click(screen.getByRole('button', { name: 'Go' }))

    expect(onClick).toHaveBeenCalledOnce()
  })
})
