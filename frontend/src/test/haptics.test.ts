import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { handleInteractiveClick } from '../lib/haptics'

// App-mode tap path: the bridge is present, so the delegate must fire it.
vi.mock('../lib/api', () => ({ isApp: () => true }))

function setup() {
  document.addEventListener('click', handleInteractiveClick, true)
  return () => document.removeEventListener('click', handleInteractiveClick, true)
}

function makeClickable(tag: 'a' | 'button', attrs: Record<string, string> = {}): HTMLElement {
  const el = document.createElement(tag)
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v)
  el.textContent = 'x'
  document.body.appendChild(el)
  return el
}

describe('handleInteractiveClick', () => {
  let buttonHaptic: ReturnType<typeof vi.fn>
  let cleanup: () => void

  beforeEach(() => {
    cleanup = setup()
    buttonHaptic = vi.fn()
    ;(window as unknown as Record<string, unknown>).GourlBridge = { buttonHaptic }
  })

  afterEach(() => {
    cleanup()
    delete (window as unknown as Record<string, unknown>).GourlBridge
    localStorage.removeItem('gourl-haptics')
    document.body.innerHTML = ''
  })

  it('fires the haptic on a plain button', () => {
    makeClickable('button').dispatchEvent(new MouseEvent('click', { bubbles: true }))
    expect(buttonHaptic).toHaveBeenCalledOnce()
  })

  it('fires the haptic on an in-app anchor (same origin, no target)', () => {
    makeClickable('a', { href: '/admin/links' }).dispatchEvent(new MouseEvent('click', { bubbles: true }))
    expect(buttonHaptic).toHaveBeenCalledOnce()
  })

  it('skips switches and checkboxes (dedicated toggles)', () => {
    makeClickable('button', { role: 'switch' }).dispatchEvent(new MouseEvent('click', { bubbles: true }))
    makeClickable('button', { role: 'checkbox' }).dispatchEvent(new MouseEvent('click', { bubbles: true }))
    expect(buttonHaptic).not.toHaveBeenCalled()
  })

  it('skips external anchors (target=_blank or a different origin)', () => {
    makeClickable('a', { href: 'https://example.com/x', target: '_blank' }).dispatchEvent(
      new MouseEvent('click', { bubbles: true }),
    )
    makeClickable('a', { href: 'https://example.com/x' }).dispatchEvent(new MouseEvent('click', { bubbles: true }))
    expect(buttonHaptic).not.toHaveBeenCalled()
  })

  it('does not fire when the click lands outside any control', () => {
    const div = document.createElement('div')
    document.body.appendChild(div)
    div.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    expect(buttonHaptic).not.toHaveBeenCalled()
  })

  it('stays silent when the haptics setting is off', () => {
    localStorage.setItem('gourl-haptics', '0')
    makeClickable('button').dispatchEvent(new MouseEvent('click', { bubbles: true }))
    expect(buttonHaptic).not.toHaveBeenCalled()
  })
})
