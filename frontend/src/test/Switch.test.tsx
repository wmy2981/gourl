import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Switch } from '../components/ui'

describe('Switch', () => {
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
})
