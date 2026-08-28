import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Button } from '../components/ui'

// Tap haptics moved to the global click delegate (lib/haptics.ts) — Button
// itself only forwards the click now; the delegate path is covered by
// haptics.test.ts.
describe('Button', () => {
  it('calls onClick', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()

    render(<Button onClick={onClick}>Go</Button>)
    await user.click(screen.getByRole('button', { name: 'Go' }))

    expect(onClick).toHaveBeenCalledOnce()
  })
})
