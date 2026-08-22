import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach, beforeEach } from 'vitest'

afterEach(() => cleanup())

// jsdom lacks the DOM APIs CodeMirror needs: ResizeObserver (size tracking)
// and Range client rects (the measure path after focus changes).
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

beforeEach(() => {
  globalThis.ResizeObserver = ResizeObserverStub
  const rect = { top: 0, left: 0, right: 0, bottom: 0, width: 0, height: 0 }
  Range.prototype.getClientRects = (() => [rect]) as unknown as Range['getClientRects']
  Range.prototype.getBoundingClientRect = () => rect as DOMRect
})
