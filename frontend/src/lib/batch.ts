// Batch-create line parser: one item per line, strict syntax
//
//   [code](date)url
//
// where both brackets are optional but must follow the exact shape when
// present: `[code]` first, then `(date)`, then the url. Dates accept
// yyyy/MM/dd and yyyy-MM-dd (month/day may be 1-2 digits) and resolve to
// unix seconds at local midnight. Lines starting with # (after trimming) are
// comments and are skipped by callers; so are empty lines.

export interface ParsedBatchItem {
  url: string
  code?: string
  expires_at?: number
}

export interface BatchLineResult {
  ok: boolean
  /** Machine-readable reason for the UI to translate: invalidSyntax | invalidUrl | invalidDate */
  error?: string
  item?: ParsedBatchItem
}

// Absolute-URL check mirroring the backend's isAbsoluteURL (internal/api/
// links.go): any non-empty scheme with a host or path — http(s), tcp,
// openapp, mailto: all qualify; scheme-less strings do not.
function isAbsoluteUrl(s: string): boolean {
  let u: URL
  try {
    u = new URL(s)
  } catch {
    return false
  }
  return u.protocol !== '' && (u.host !== '' || u.pathname !== '')
}

/** Parses yyyy/MM/dd or yyyy-MM-dd (loose month/day) into local-midnight unix seconds. */
export function parseBatchDate(raw: string): number | null {
  const m = raw.trim().match(/^(\d{4})[/-](\d{1,2})[/-](\d{1,2})$/)
  if (!m) return null
  const [y, mo, d] = [Number(m[1]), Number(m[2]), Number(m[3])]
  if (mo < 1 || mo > 12 || d < 1 || d > 31) return null
  const dt = new Date(y, mo - 1, d)
  if (dt.getFullYear() !== y || dt.getMonth() !== mo - 1 || dt.getDate() !== d) return null
  return Math.floor(dt.getTime() / 1000)
}

export function parseBatchLine(line: string): BatchLineResult {
  let rest = line.trim()
  if (rest === '' || rest.startsWith('#')) {
    return { ok: false, error: 'skip' }
  }

  let code: string | undefined
  if (rest.startsWith('[')) {
    const close = rest.indexOf(']')
    if (close === -1) return { ok: false, error: 'invalidSyntax' }
    code = rest.slice(1, close).trim()
    if (!code) return { ok: false, error: 'invalidSyntax' }
    rest = rest.slice(close + 1)
  }

  let expiresAt: number | undefined
  if (rest.startsWith('(')) {
    const close = rest.indexOf(')')
    if (close === -1) return { ok: false, error: 'invalidSyntax' }
    const raw = rest.slice(1, close).trim()
    const unix = raw ? parseBatchDate(raw) : null
    if (!raw || unix === null) return { ok: false, error: 'invalidDate' }
    expiresAt = unix
    rest = rest.slice(close + 1)
  }

  const url = rest.trim()
  if (!url) return { ok: false, error: 'invalidSyntax' }
  if (!isAbsoluteUrl(url)) return { ok: false, error: 'invalidUrl' }

  return { ok: true, item: { url, code, expires_at: expiresAt } }
}
