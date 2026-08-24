import { describe, expect, it, vi } from 'vitest'
import { act, render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import Layout from '../components/Layout'
import { ToastProvider } from '../components/ui'
import '../lib/i18n'

// jsdom reports a 768px viewport by default — the drawer gesture only exists
// on phones (below md), so every test pins a narrow window first. jsdom also
// lacks matchMedia, which the Layout theme hook needs.
const phoneViewport = () => {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
  Object.defineProperty(window, 'innerHeight', { configurable: true, value: 844 })
  if (!window.matchMedia) {
    window.matchMedia = ((query: string) => ({
      matches: false,
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
    })) as unknown as typeof window.matchMedia
  }
}

// jsdom lacks the Touch constructor: a plain Event carrying the touch list
// is enough — the handlers only read e.touches[0].clientX/Y.
function touch(el: Element, type: string, x: number, y = 0) {
  const point = { identifier: 1, clientX: x, clientY: y, target: el }
  act(() => {
    el.dispatchEvent(
      Object.assign(new Event(type, { bubbles: true, cancelable: true }), { touches: [point] }),
    )
  })
}

function renderLayout() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={['/admin']}>
        <Routes>
          <Route path="/admin" element={<Layout />}>
            <Route index element={<p>dashboard page</p>} />
          </Route>
        </Routes>
        <ToastProvider>{null}</ToastProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// The gesture handlers live on Layout's own root div; jsdom's body is not
// part of that React tree.
const touchRoot = () => document.querySelector('.min-h-screen')!

// The drawer mounts only once the gesture commits; the menu button is the
// always-visible open affordance.
const menuButton = () => screen.getByRole('button', { name: 'Menu' })
const drawerRoot = () => document.querySelector('.fixed.inset-0.z-50')

describe('mobile drawer first-tap fix', () => {
  it('keeps the drawer closed for taps in the edge zone', () => {
    phoneViewport()
    renderLayout()
    // A tap inside the right-30% zone: touchstart arms the gesture (refs
    // only), touchend releases it. No drag → nothing mounts.
    touch(touchRoot(), 'touchstart', 350)
    expect(drawerRoot()).toBeNull()
    touch(touchRoot(), 'touchend', 350)
    expect(drawerRoot()).toBeNull()
  })

  it('opens the drawer on a committed swipe and closes it with one backdrop tap', () => {
    phoneViewport()
    vi.useFakeTimers()
    try {
      renderLayout()

      const root = touchRoot()
      // Commit the horizontal drag (>8px left) and release past the halfway
      // mark: the drawer stays mounted in its settled-open position.
      touch(root, 'touchstart', 380)
      touch(root, 'touchmove', 340)
      touch(root, 'touchmove', 200)
      touch(root, 'touchend', 200)
      act(() => {
        vi.advanceTimersByTime(400) // let the release settle animation finish
      })
      expect(drawerRoot()).not.toBeNull()

      // First tap after opening must work immediately: the backdrop click
      // path closes the drawer (the exit animation unmounts it after 320ms).
      const backdrop = drawerRoot()!.firstElementChild as HTMLElement
      act(() => {
        backdrop.dispatchEvent(new MouseEvent('click', { bubbles: true }))
      })
      act(() => {
        vi.advanceTimersByTime(400)
      })
      expect(drawerRoot()).toBeNull()
    } finally {
      vi.useRealTimers()
    }
  })

  it('does not restyle anything between touchstart and the commit threshold', () => {
    phoneViewport()
    vi.useFakeTimers()
    try {
      renderLayout()
      // Open through the button (the settled-open state), then arm a new
      // gesture on the edge zone without committing it: no re-render may
      // touch the drawer's inline transition style, or the WebView cancels
      // the tap.
      act(() => {
        menuButton().dispatchEvent(new MouseEvent('click', { bubbles: true }))
      })
      act(() => {
        vi.advanceTimersByTime(16) // openMobile settles on the next frame
      })
      const panel = drawerRoot()!.querySelector('[style*="transform"]') as HTMLElement
      const before = panel.getAttribute('style')
      const root = touchRoot()
      touch(root, 'touchstart', 350)
      touch(root, 'touchmove', 349) // below the 8px commit threshold
      touch(root, 'touchend', 349)
      expect(panel.getAttribute('style')).toBe(before)
    } finally {
      vi.useRealTimers()
    }
  })
})
