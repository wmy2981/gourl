import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import DateInput from '../components/DateInput'

function renderDate(value: string) {
  const onChange = vi.fn()
  render(<DateInput value={value} onChange={onChange} ariaLabel="expiry" />)
  return { onChange }
}

describe('DateInput', () => {
  it('renders the initial value in the fixed yyyy/MM/dd format', () => {
    renderDate('2026-08-16')
    expect(screen.getByLabelText('expiry')).toHaveValue('2026/08/16')
  })

  it('reports ISO when typed as yyyy/MM/dd', async () => {
    const user = userEvent.setup()
    const { onChange } = renderDate('')
    const input = screen.getByLabelText('expiry')
    await user.clear(input)
    await user.type(input, '2026/12/31')
    expect(onChange).toHaveBeenLastCalledWith('2026-12-31')
  })

  it('accepts yyyy-MM-dd typing too', async () => {
    const user = userEvent.setup()
    const { onChange } = renderDate('')
    const input = screen.getByLabelText('expiry')
    await user.clear(input)
    await user.type(input, '2026-12-31')
    expect(onChange).toHaveBeenLastCalledWith('2026-12-31')
  })

  it('rejects invalid dates as null', async () => {
    const user = userEvent.setup()
    const { onChange } = renderDate('')
    const input = screen.getByLabelText('expiry')
    await user.clear(input)
    await user.type(input, '2026/13/45')
    expect(onChange).toHaveBeenLastCalledWith(null)
  })

  it('clears to empty (never expires) via the clear button', async () => {
    const user = userEvent.setup()
    const { onChange } = renderDate('2026-08-16')
    await user.click(screen.getByRole('button', { name: 'Clear' }))
    expect(onChange).toHaveBeenLastCalledWith('')
    expect(screen.getByLabelText('expiry')).toHaveValue('')
  })

  it('opens the native picker from the calendar button', async () => {
    const user = userEvent.setup()
    const showPicker = vi.fn()
    Object.defineProperty(HTMLInputElement.prototype, 'showPicker', {
      configurable: true,
      value: showPicker,
    })
    renderDate('')
    await user.click(screen.getByRole('button', { name: 'Pick a date' }))
    expect(showPicker).toHaveBeenCalledTimes(1)
  })
})
