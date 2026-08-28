import { describe, expect, it, vi } from 'vitest'
import { shareUrl } from '../lib/share'

function stubShare(impl: ((data: { url?: string }) => Promise<void>) | undefined) {
  Object.defineProperty(navigator, 'share', { value: impl, configurable: true })
}

describe('shareUrl', () => {
  it('reports unsupported when the Web Share API is absent (plain http)', async () => {
    delete (navigator as { share?: unknown }).share
    expect(await shareUrl('https://host/abc')).toBe('unsupported')
  })

  it('shares the url when the sheet closes normally', async () => {
    stubShare(vi.fn().mockResolvedValue(undefined))
    expect(await shareUrl('https://host/abc')).toBe('shared')
  })

  it('treats a user dismiss (AbortError) as cancelled, not failed', async () => {
    stubShare(vi.fn().mockRejectedValue(new DOMException('aborted', 'AbortError')))
    expect(await shareUrl('https://host/abc')).toBe('cancelled')
  })

  it('reports failed on other errors', async () => {
    stubShare(vi.fn().mockRejectedValue(new Error('boom')))
    expect(await shareUrl('https://host/abc')).toBe('failed')
  })

  it('sends only the url in the share payload', async () => {
    const share = vi.fn().mockResolvedValue(undefined)
    stubShare(share)
    await shareUrl('https://host/abc')
    expect(share).toHaveBeenCalledWith({ url: 'https://host/abc' })
  })
})

describe('shareUrl in app mode', () => {
  function setAppMode(on: boolean) {
    if (on) {
      ;(window as unknown as Record<string, unknown>).Capacitor = { isNativePlatform: () => true }
    } else {
      delete (window as unknown as Record<string, unknown>).Capacitor
    }
  }

  it('routes through the native bridge and reports shared', async () => {
    setAppMode(true)
    const bridgeShare = vi.fn().mockReturnValue(true)
    ;(window as unknown as Record<string, unknown>).GourlBridge = { share: bridgeShare }
    try {
      expect(await shareUrl('https://host/abc')).toBe('shared')
      expect(bridgeShare).toHaveBeenCalledWith('https://host/abc')
    } finally {
      setAppMode(false)
      delete (window as unknown as Record<string, unknown>).GourlBridge
    }
  })

  it('reports unsupported when the APK predates the share bridge', async () => {
    setAppMode(true)
    try {
      expect(await shareUrl('https://host/abc')).toBe('unsupported')
    } finally {
      setAppMode(false)
    }
  })
})
