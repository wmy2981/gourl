import { describe, expect, it } from 'vitest'
import { parseCSV } from '../lib/csv'

describe('parseCSV', () => {
  it('parses simple rows', () => {
    const rows = parseCSV('code,url\na,https://e.com/1\nb,https://e.com/2')
    expect(rows).toEqual([
      { code: 'a', url: 'https://e.com/1' },
      { code: 'b', url: 'https://e.com/2' },
    ])
  })

  it('handles quoted fields with commas and escaped quotes', () => {
    const rows = parseCSV('code,title\n"a,b","say ""hi"""')
    expect(rows).toEqual([{ code: 'a,b', title: 'say "hi"' }])
  })

  it('handles CRLF line endings', () => {
    const rows = parseCSV('code,url\r\nx,https://e.com/x\r\n')
    expect(rows).toEqual([{ code: 'x', url: 'https://e.com/x' }])
  })

  it('returns empty for blank input', () => {
    expect(parseCSV('')).toEqual([])
    expect(parseCSV('\n\n')).toEqual([])
  })
})
