import { describe, expect, it } from 'vitest'
import { ApiError } from '../lib/api'
import { apiErrorMessage } from '../lib/apiError'
import en from '../locales/en.json'
import zh from '../locales/zh.json'

// Minimal stand-in for i18next's t: reads the flat "a.b" path out of one
// dictionary and applies {{param}} interpolation like i18next does.
function makeT(dict: Record<string, unknown>) {
  return (key: string, opts?: Record<string, unknown>) => {
    const value = key
      .split('.')
      .reduce<unknown>((o, k) => (o as Record<string, unknown>)?.[k], dict)
    if (typeof value !== 'string') throw new Error(`missing key: ${key}`)
    return value.replace(/\{\{(\w+)\}\}/g, (_, name) => String(opts?.[name] ?? `{{${name}}}`))
  }
}

describe('apiErrorMessage', () => {
  it('maps a known validation code with params in both languages', () => {
    const err = new ApiError(400, 'short_code_length_range', 'short_code_length must be between 2 and 64, got 1',
      { min: 2, max: 64, got: 1 })
    expect(apiErrorMessage(err, makeT(en))).toBe('Random code length must be between 2 and 64 (got 1)')
    expect(apiErrorMessage(err, makeT(zh))).toBe('随机短码长度必须在 2 到 64 之间（当前为 1）')
  })

  it('interpolates string params', () => {
    const err = new ApiError(400, 'invalid_log_level', 'log_level must be debug, info, warning or error, got "verbose"',
      { got: 'verbose' })
    expect(apiErrorMessage(err, makeT(zh))).toBe('日志级别必须是 debug、info、warning 或 error（当前为 verbose）')
  })

  it('falls back to the backend message for unknown codes', () => {
    const err = new ApiError(400, 'some_future_code', 'something specific happened')
    expect(apiErrorMessage(err, makeT(en))).toBe('something specific happened')
  })

  it('falls back when params are missing entirely', () => {
    const err = new ApiError(409, 'token_taken', 'token already exists')
    expect(apiErrorMessage(err, makeT(en))).toBe('This token already exists')
  })
})
