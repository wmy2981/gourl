import { afterEach, describe, expect, it } from 'vitest'
import { assetUrl, linkUrls, setServerConfig } from '../lib/api'

afterEach(() => {
  setServerConfig(null)
})

describe('assetUrl', () => {
  it('returns the path unchanged without a stored server config (web mode)', () => {
    expect(assetUrl('/assets/custom-icon.svg')).toBe('/assets/custom-icon.svg')
  })

  it('resolves against the connected server in app mode, trimming trailing slashes', () => {
    setServerConfig({ url: 'http://192.168.1.10:8080/', token: 'x' })
    expect(assetUrl('/assets/custom-icon.svg')).toBe('http://192.168.1.10:8080/assets/custom-icon.svg')
  })
})

describe('linkUrls', () => {
  const cfg = {
    base_url: 'https://s.example.com',
    extra_base_urls: ['https://e.example.com'],
  } as never

  it('joins normal codes with a separator across all base URLs', () => {
    expect(linkUrls('abc', cfg)).toEqual([
      'https://s.example.com/abc',
      'https://e.example.com/abc',
    ])
  })

  it('maps the bare "/" code to the site roots without a double slash', () => {
    expect(linkUrls('/', cfg)).toEqual([
      'https://s.example.com',
      'https://e.example.com',
    ])
  })
})
