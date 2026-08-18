// Minimal fetch wrapper for the gourl REST API. A 401 redirects to login;
// other failures throw ApiError with the server's error code/message.
//
// Mobile-app mode: the Capacitor app stores a server URL + API token here
// and talks to a remote instance with a Bearer header instead of a session
// cookie. Web console mode is unaffected — no stored config means relative
// URLs + cookies, exactly as before.

/** Remote server connection for the mobile app (localStorage `gourl-server`). */
export interface ServerConfig {
  url: string
  token: string
}

const SERVER_KEY = 'gourl-server'

export function getServerConfig(): ServerConfig | null {
  try {
    const raw = localStorage.getItem(SERVER_KEY)
    if (!raw) return null
    const cfg = JSON.parse(raw) as ServerConfig
    return cfg.url && cfg.token ? cfg : null
  } catch {
    return null
  }
}

export function setServerConfig(cfg: ServerConfig | null) {
  if (cfg) localStorage.setItem(SERVER_KEY, JSON.stringify(cfg))
  else localStorage.removeItem(SERVER_KEY)
}

/** Reports whether the SPA runs inside the Capacitor app (token mode). */
export function isApp(): boolean {
  return typeof window !== 'undefined' && 'Capacitor' in window
}

export interface ApiErrorBody {
  error: { code: string; message: string }
}

export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

export interface Link {
  id: number
  code: string
  url: string
  title: string
  description: string
  expires_at: number
  click_count: number
  created_at: number
  updated_at: number
}

// linkUrls assembles every complete short URL for a code from the config
// (mirroring the old backend fullURLs): the base URL — or the current
// location when unset — plus every extra base URL, deduplicated, trailing
// slashes trimmed.
export function linkUrls(code: string, cfg: AppConfig): string[] {
  const bases: string[] = []
  const push = (base: string) => {
    const trimmed = base.trim().replace(/\/+$/, '')
    if (trimmed && !bases.includes(trimmed)) bases.push(trimmed)
  }
  push(cfg.base_url || `${location.protocol}//${location.host}`)
  for (const extra of cfg.extra_base_urls) push(extra)
  return bases.map((b) => `${b}/${code}`)
}

export interface LinkListResponse {
  links: Link[]
  total: number
  page: number
  page_size: number
}

export interface UABlock {
  id: number
  pattern: string
  created_at?: number
}

export interface TokenInfo {
  id: number
  token: string
  note: string
  created_at: number
}

export interface SiteInfo {
  name: string
  title: string
  keywords: string
  description: string
}

export interface AppConfig {
  site: SiteInfo
  short_code_length: number
  base_url: string
  extra_base_urls: string[]
  reserved_codes: string[]
  ua_blocks: string[]
  ip_blocks: string[]
  login_rate_max_attempts: number
  login_rate_lock_seconds: number
  session_ttl_minutes: number
  link_rate_per_second: number
  log_level: string
  icon: string
}

export interface DashboardData {
  links_total: number
  clicks_total: number
  clicks_today: number
  daily: { date: string; count: number }[]
}

export interface LogRecord {
  time: string
  level: 'debug' | 'info' | 'warn' | 'error' | ''
  message: string
  attrs?: Record<string, unknown>
}

export interface LogHistoryResponse {
  available: boolean
  records: LogRecord[]
}

export interface BatchCreateResult {
  index: number
  url: string
  status: 'created' | 'updated' | 'skipped' | 'error'
  code?: string
  error_code?: string
  error_message?: string
}

export interface BatchCreateResponse {
  created: number
  failed: number
  /** Per-status counts and code lists: created, skipped, updated, failed. */
  succeeded: number
  skipped: number
  updated: number
  failed_codes: string[]
  skipped_codes: string[]
  updated_codes: string[]
  results: BatchCreateResult[]
}

/** Import item: url is required; the rest is optional and mirrors the export fields. click_count is dropped on import; deleted: true skips the item (re-imported export dumps). */
export interface ImportItem {
  url: string
  code?: string
  title?: string
  description?: string
  expires_at?: number | string
  created_at?: number
  deleted?: boolean
}

const JSON_HEADERS = { 'Content-Type': 'application/json' }

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const server = getServerConfig()
  // FormData bodies must keep the browser-set multipart boundary; forcing the
  // JSON content type would make the server reject the file field.
  const isForm = typeof FormData !== 'undefined' && init.body instanceof FormData
  const headers: Record<string, string> = isForm
    ? { ...(init.headers as Record<string, string> | undefined) }
    : { ...JSON_HEADERS, ...(init.headers as Record<string, string> | undefined) }
  if (server) headers.Authorization = `Bearer ${server.token}`
  const res = await fetch(server ? server.url.replace(/\/+$/, '') + path : path, {
    credentials: server ? 'omit' : 'same-origin',
    ...init,
    headers,
  })
  if (res.status === 401) {
    if (server) {
      // Token mode: never bounce to the login/setup pages — the connect
      // screen owns re-authentication (bad token, revoked token, …).
      throw new ApiError(401, 'unauthorized', 'token invalid or expired')
    }
    // Not authenticated (or session expired): back to login.
    if (window.location.pathname !== '/admin/login') {
      window.location.href = '/admin/login'
    }
    throw new ApiError(401, 'unauthorized', 'authentication required')
  }
  if (!res.ok) {
    let code = 'unknown'
    let message = `HTTP ${res.status}`
    try {
      const body = (await res.json()) as ApiErrorBody
      code = body.error?.code ?? code
      message = body.error?.message ?? message
    } catch {
      // non-JSON error body
    }
    throw new ApiError(res.status, code, message)
  }
  if (res.status === 204) {
    return undefined as T
  }
  return (await res.json()) as T
}

export const api = {
  login: (password: string) =>
    request<{ ok: boolean }>('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ password }),
    }),
  logout: () =>
    request<{ ok: boolean }>('/api/v1/auth/logout', { method: 'POST' }),
  changePassword: (oldPassword: string, newPassword: string) =>
    request<{ ok: boolean }>('/api/v1/auth/change-password', {
      method: 'POST',
      body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
    }),
  setupAdmin: (code: string, password: string) =>
    request<{ ok: boolean }>('/api/v1/auth/setup', {
      method: 'POST',
      body: JSON.stringify({ code, password }),
    }),
  authStatus: () => request<{ configured: boolean }>('/api/v1/auth/status'),
  health: () => request<{ name: string }>('/api/v1/health'),

  listLinks: (params: Record<string, string | number | undefined>) => {
    const q = new URLSearchParams()
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== '') q.set(k, String(v))
    }
    const qs = q.toString()
    return request<LinkListResponse>(`/api/v1/links${qs ? `?${qs}` : ''}`)
  },
  createLink: (body: { url: string; code?: string; expires_at?: number; title?: string; description?: string }) =>
    request<Link>('/api/v1/links', { method: 'POST', body: JSON.stringify(body) }),
  batchCreate: (items: ImportItem[], conflict: 'error' | 'skip' | 'update' = 'error') =>
    request<BatchCreateResponse>('/api/v1/links/batch', {
      method: 'POST',
      body: JSON.stringify({ conflict, items }),
    }),
  getLink: (code: string) => request<Link>(`/api/v1/links/${encodePath(code)}`),
  updateLink: (code: string, body: Record<string, unknown>) =>
    request<Link>(`/api/v1/links/${encodePath(code)}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  deleteLink: (code: string) =>
    request<void>(`/api/v1/links/${encodePath(code)}`, { method: 'DELETE' }),
  deleteLinks: (codes: string[]) =>
    request<{ deleted: number }>('/api/v1/links', {
      method: 'DELETE',
      body: JSON.stringify({ codes }),
    }),
  expiredCount: () => request<{ count: number }>('/api/v1/links/expired'),
  deleteExpired: () =>
    request<{ deleted: number }>('/api/v1/links/expired', { method: 'DELETE' }),
  exportCsv: async () => {
    // The CSV endpoint is not JSON, so it bypasses request(): fetch the blob
    // directly (absolute URL + Bearer in token mode, cookie in the web console).
    const server = getServerConfig()
    const res = await fetch(
      server ? `${server.url.replace(/\/+$/, '')}/api/v1/export.csv` : '/api/v1/export.csv',
      {
        credentials: server ? 'omit' : 'same-origin',
        headers: server ? { Authorization: `Bearer ${server.token}` } : {},
      },
    )
    if (!res.ok) throw new ApiError(res.status, 'unknown', `HTTP ${res.status}`)
    return res.blob()
  },
  exportJson: () => request<Record<string, unknown>[]>('/api/v1/export.json'),
  logHistory: (limit = 200, offset = 0) =>
    request<LogHistoryResponse>(`/api/v1/logs?limit=${limit}&offset=${offset}`),
  logStream: (onLog: (rec: LogRecord) => void, onError?: () => void) => {
    const server = getServerConfig()
    const url = server
      ? `${server.url.replace(/\/+$/, '')}/api/v1/logs/stream`
      : '/api/v1/logs/stream'
    if (server) {
      // Token mode: EventSource cannot send an Authorization header, so parse
      // the SSE stream over fetch. Frames are `event: log\ndata: {...}\n\n`.
      const abort = new AbortController()
      fetch(url, { headers: { Authorization: `Bearer ${server.token}` }, signal: abort.signal })
        .then((res) => {
          if (!res.ok || !res.body) throw new Error(`HTTP ${res.status}`)
          const reader = res.body.getReader()
          const decoder = new TextDecoder()
          let buf = ''
          const pump = (): Promise<void> =>
            reader
              .read()
              .then(({ done, value }) => {
                if (done) {
                  onError?.()
                  return
                }
                buf += decoder.decode(value, { stream: true })
                let idx: number
                while ((idx = buf.indexOf('\n\n')) >= 0) {
                  const dataLine = buf
                    .slice(0, idx)
                    .split('\n')
                    .find((l) => l.startsWith('data:'))
                  buf = buf.slice(idx + 2)
                  if (dataLine) {
                    try {
                      onLog(JSON.parse(dataLine.slice(5).trim()) as LogRecord)
                    } catch {
                      // malformed frame: ignore
                    }
                  }
                }
                return pump()
              })
              .catch(() => onError?.())
          return pump()
        })
        .catch(() => onError?.())
      return { close: () => abort.abort() }
    }
    const es = new EventSource(url)
    es.addEventListener('log', (e) => {
      try {
        onLog(JSON.parse((e as MessageEvent).data) as LogRecord)
      } catch {
        // malformed frame: ignore
      }
    })
    es.onerror = () => onError?.()
    return es
  },

  uaBlocks: () => request<{ ua_blocks: UABlock[] }>('/api/v1/ua-blocks'),
  addUABlock: (pattern: string) =>
    request<{ id: number }>('/api/v1/ua-blocks', {
      method: 'POST',
      body: JSON.stringify({ pattern }),
    }),
  deleteUABlock: (id: number) =>
    request<void>(`/api/v1/ua-blocks/${id}`, { method: 'DELETE' }),

  tokens: () => request<{ tokens: TokenInfo[] }>('/api/v1/tokens'),
  createToken: (note: string) =>
    request<{ id: number; token: string }>('/api/v1/tokens', {
      method: 'POST',
      body: JSON.stringify({ note }),
    }),
  deleteToken: (id: number) =>
    request<void>(`/api/v1/tokens/${id}`, { method: 'DELETE' }),

  getConfig: () => request<AppConfig>('/api/v1/config'),
  updateConfig: (cfg: AppConfig) =>
    request<AppConfig>('/api/v1/config', {
      method: 'PUT',
      body: JSON.stringify(cfg),
    }),
  uploadIcon: (file: File) => {
    const form = new FormData()
    form.append('icon', file)
    return request<{ icon: string }>('/api/v1/icon', { method: 'POST', body: form })
  },
  deleteIcon: () => request<{ icon: string }>('/api/v1/icon', { method: 'DELETE' }),

  dashboard: () => request<DashboardData>('/api/v1/dashboard'),
}

// encodePath preserves '/' inside multi-level codes (link1/link2) while
// encoding everything else safely.
function encodePath(code: string): string {
  return code
    .split('/')
    .map((seg) => encodeURIComponent(seg))
    .join('/')
}
