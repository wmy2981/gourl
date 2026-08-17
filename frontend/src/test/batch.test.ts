import { describe, expect, it } from 'vitest'
import { parseBatchLine, parseBatchDate } from '../lib/batch'

describe('parseBatchDate', () => {
  it('parses yyyy/MM/dd and yyyy-MM-dd at local midnight', () => {
    const want = Math.floor(new Date(2030, 0, 2).getTime() / 1000)
    expect(parseBatchDate('2030/01/02')).toBe(want)
    expect(parseBatchDate('2030-01-02')).toBe(want)
    expect(parseBatchDate('2030/1/2')).toBe(want)
  })

  it('rejects impossible dates', () => {
    expect(parseBatchDate('2030/13/01')).toBeNull()
    expect(parseBatchDate('2030/02/30')).toBeNull()
    expect(parseBatchDate('not-a-date')).toBeNull()
  })
})

describe('parseBatchLine', () => {
  it('accepts a bare url', () => {
    const r = parseBatchLine('https://example.com/a')
    expect(r.ok).toBe(true)
    expect(r.item).toEqual({ url: 'https://example.com/a' })
  })

  it('accepts code and date', () => {
    const r = parseBatchLine('[mycode](2030/12/31)https://example.com/a')
    expect(r.ok).toBe(true)
    expect(r.item?.code).toBe('mycode')
    expect(r.item?.expires_at).toBe(parseBatchDate('2030/12/31'))
    expect(r.item?.url).toBe('https://example.com/a')
  })

  it('accepts only a date', () => {
    const r = parseBatchLine('(2030/12/31)https://example.com/a')
    expect(r.ok).toBe(true)
    expect(r.item?.code).toBeUndefined()
    expect(r.item?.expires_at).toBe(parseBatchDate('2030/12/31'))
  })

  it('skips comments and empty lines', () => {
    expect(parseBatchLine('# a comment').error).toBe('skip')
    expect(parseBatchLine('   ').error).toBe('skip')
  })

  it('rejects a missing closing bracket', () => {
    expect(parseBatchLine('[abc(2030/12/31)https://example.com/a').error).toBe('invalidSyntax')
  })

  it('rejects an empty code', () => {
    expect(parseBatchLine('[](2030/12/31)https://example.com/a').error).toBe('invalidSyntax')
  })

  it('rejects a bad date', () => {
    expect(parseBatchLine('[abc](2030/13/01)https://example.com/a').error).toBe('invalidDate')
  })

  it('rejects a non-http url', () => {
    expect(parseBatchLine('ftp://example.com/a').error).toBe('invalidUrl')
  })

  it('rejects a missing url', () => {
    expect(parseBatchLine('[abc](2030/12/31)').error).toBe('invalidSyntax')
  })
})
