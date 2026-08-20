import { afterEach, describe, expect, it } from 'vitest'
import { assetUrl, setServerConfig } from '../lib/api'

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
