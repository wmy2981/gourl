import { afterEach, describe, expect, it } from 'vitest'
import { applyNoSelect, noSelectEnabled, setNoSelect } from '../lib/appSettings'

afterEach(() => {
  localStorage.removeItem('gourl-no-select')
  document.documentElement.classList.remove('no-select')
})

describe('noSelect app setting', () => {
  it('defaults to off', () => {
    expect(noSelectEnabled()).toBe(false)
  })

  it('toggles the html.no-select class immediately', () => {
    setNoSelect(true)
    expect(noSelectEnabled()).toBe(true)
    expect(document.documentElement).toHaveClass('no-select')

    setNoSelect(false)
    expect(noSelectEnabled()).toBe(false)
    expect(document.documentElement).not.toHaveClass('no-select')
  })

  it('applies the stored setting on applyNoSelect', () => {
    localStorage.setItem('gourl-no-select', '1')
    applyNoSelect()
    expect(document.documentElement).toHaveClass('no-select')
  })
})
