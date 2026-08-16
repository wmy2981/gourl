// Minimal fetch wrapper for the gourl REST API. A 401 redirects to login;
// other failures throw ApiError with the server's error code/message.

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
  code: string
  url: string
  title: string
  description: string
  expires_at: number
  click_count: number
  created_at: number
  updated_at: number
  urls: string[]
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
  header: string
  footer: string
}

export interface AppConfig {
  site: SiteInfo
  short_code_length: number
  base_url: string
  extra_base_urls: string[]
  reserved_codes: string[]
  icon: string
}

export interface DashboardData {
  links_total: number
  clicks_total: number
  clicks_today: number
  daily: { date: string; count: number }[]
}

const JSON_HEADERS = { 'Content-Type': 'application/json' }

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    credentials: 'same-origin',
    ...init,
    headers: { ...JSON_HEADERS, ...(init.headers ?? {}) },
  })
  if (res.status === 401) {
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
  health: () => request<Record<string, unknown>>('/api/v1/health'),

  listLinks: (params: Record<string, string | number | undefined>) => {
    const q = new URLSearchParams()
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== '') q.set(k, String(v))
    }
    const qs = q.toString()
    return request<LinkListResponse>(`/api/v1/links${qs ? `?${qs}` : ''}`)
  },
  createLink: (body: { url: string; code?: string; expires_at?: number }) =>
    request<Link>('/api/v1/links', { method: 'POST', body: JSON.stringify(body) }),
  batchCreate: (items: { url: string; code?: string; expires_at?: number }[]) =>
    request<{ created: number; failed: number; results: unknown[] }>(
      '/api/v1/links/batch',
      { method: 'POST', body: JSON.stringify(items) },
    ),
  getLink: (code: string) => request<Link>(`/api/v1/links/${encodePath(code)}`),
  updateLink: (code: string, body: Record<string, unknown>) =>
    request<Link>(`/api/v1/links/${encodePath(code)}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  deleteLink: (code: string) =>
    request<void>(`/api/v1/links/${encodePath(code)}`, { method: 'DELETE' }),
  exportCsv: () => request<Blob>('/api/v1/export.csv'),
  exportJson: () => request<Link[]>('/api/v1/export.json'),

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
