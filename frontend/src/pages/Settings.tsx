import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { KeyRound, PlugZap, Plus, Trash2, Upload } from 'lucide-react'
import { api, ApiError, getServerConfig, isApp, setServerConfig, type AppConfig, type TokenInfo } from '../lib/api'
import { copyText } from '../lib/clipboard'
import { Button, Card, Dialog, Input, Label, Select, Textarea, useToast } from '../components/ui'

export default function Settings() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data: cfg } = useQuery({ queryKey: ['config'], queryFn: api.getConfig })
  const [form, setForm] = useState<AppConfig | null>(null)
  const [extraUrlsText, setExtraUrlsText] = useState('')
  const [reservedText, setReservedText] = useState('')
  const [uaText, setUaText] = useState('')
  const [ipText, setIpText] = useState('')
  const [tokenNote, setTokenNote] = useState('')
  const [newToken, setNewToken] = useState('')

  useEffect(() => {
    if (cfg && !form) {
      setForm(cfg)
      setExtraUrlsText(cfg.extra_base_urls.join('\n'))
      setReservedText(cfg.reserved_codes.join(', '))
      setUaText(cfg.ua_blocks.join(', '))
      setIpText(cfg.ip_blocks.join(', '))
    }
  }, [cfg, form])

  const saveMutation = useMutation({
    mutationFn: (c: AppConfig) => api.updateConfig(c),
    onSuccess: () => {
      toast(t('settings.saved'))
      queryClient.invalidateQueries({ queryKey: ['config'] })
    },
    onError: (err: unknown) =>
      toast(err instanceof ApiError ? err.message : t('settings.saveFailed'), 'error'),
  })

  if (!form) {
    // The connection card renders before the config-dependent form: it must
    // stay visible even when the config request is slow or fails (the form
    // below falls back to a loading state).
    return (
      <div>
        <h1 className="mb-6 text-2xl font-semibold tracking-tight">{t('settings.heading')}</h1>
        <div className="flex flex-col gap-6">
          <ConnectionCard />
          <p className="py-16 text-center text-muted">{t('common.loading')}</p>
        </div>
      </div>
    )
  }

  const set = <K extends keyof AppConfig>(key: K, value: AppConfig[K]) =>
    setForm({ ...form, [key]: value })
  const setSite = (key: keyof AppConfig['site'], value: string) =>
    setForm({ ...form, site: { ...form.site, [key]: value } })

  const save = () => {
    saveMutation.mutate({
      site: form.site,
      short_code_length: form.short_code_length,
      base_url: form.base_url,
      login_rate_max_attempts: form.login_rate_max_attempts,
      login_rate_lock_seconds: form.login_rate_lock_seconds,
      session_ttl_minutes: form.session_ttl_minutes,
      link_rate_per_second: form.link_rate_per_second,
      log_level: form.log_level,
      icon: form.icon,
      extra_base_urls: extraUrlsText
        .split('\n')
        .map((s) => s.trim())
        .filter(Boolean),
      reserved_codes: reservedText
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean),
      ua_blocks: uaText
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean),
      ip_blocks: ipText
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean),
    })
  }

  const iconInput = async (file: File | undefined) => {
    if (!file) return
    try {
      const res = await api.uploadIcon(file)
      // Keep the page snapshot in sync so the reset button appears right away.
      setForm((f) => (f ? { ...f, icon: res.icon } : f))
      toast(t('settings.saved'))
      queryClient.invalidateQueries({ queryKey: ['config'] })
    } catch (err) {
      toast(err instanceof ApiError ? err.message : t('settings.saveFailed'), 'error')
    }
  }

  return (
    <div>
      <h1 className="mb-6 text-2xl font-semibold tracking-tight">{t('settings.heading')}</h1>

      <div className="flex flex-col gap-6">
        {/* App connection (Capacitor token mode only) */}
        {isApp() && <ConnectionCard />}

        {/* Site information */}
        <Card className="p-6">
          <h2 className="mb-4 text-sm font-medium text-muted">{t('settings.site')}</h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <Label htmlFor='cfg-site-name'>{t('settings.siteName')}</Label>
              <Input id='cfg-site-name' value={form.site.name} onChange={(e) => setSite('name', e.target.value)} />
            </div>
            <div>
              <Label htmlFor='cfg-site-title'>{t('settings.siteTitle')}</Label>
              <Input id='cfg-site-title' value={form.site.title} onChange={(e) => setSite('title', e.target.value)} />
            </div>
            <div>
              <Label htmlFor='cfg-keywords'>{t('settings.keywords')}</Label>
              <Input id='cfg-keywords' value={form.site.keywords} onChange={(e) => setSite('keywords', e.target.value)} />
            </div>
            <div>
              <Label htmlFor='cfg-description'>{t('settings.description')}</Label>
              <Input id='cfg-description' value={form.site.description} onChange={(e) => setSite('description', e.target.value)} />
            </div>
          </div>
        </Card>

        {/* Security */}
        <Card className="p-6">
          <h2 className="mb-2 text-sm font-medium text-muted">{t('settings.security')}</h2>
          <p className="mb-3 text-sm text-muted">{t('settings.securityHint')}</p>
          <Button variant="outline" onClick={() => navigate('/admin/change-password')}>
            <KeyRound size={15} />
            {t('settings.changePassword')}
          </Button>
        </Card>

        {/* Behavior */}
        <Card className="p-6">
          <h2 className="mb-4 text-sm font-medium text-muted">{t('settings.behavior')}</h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <Label htmlFor='cfg-code-length'>{t('settings.shortCodeLength')}</Label>
              <Input
                id='cfg-code-length'
                type="number"
                min={4}
                max={32}
                value={form.short_code_length}
                onChange={(e) => set('short_code_length', Number(e.target.value))}
              />
            </div>
            <div>
              <Label htmlFor='cfg-base-url'>{t('settings.baseUrl')}</Label>
              <Input id='cfg-base-url' value={form.base_url} onChange={(e) => set('base_url', e.target.value)} placeholder="https://s.example.com" />
              <p className="mt-1 text-xs text-muted">{t('settings.baseUrlHint')}</p>
            </div>
            <div className="sm:col-span-2">
              <Label htmlFor="cfg-extra-urls">{t('settings.extraBaseUrls')}</Label>
              <Textarea
                id="cfg-extra-urls"
                rows={3}
                value={extraUrlsText}
                onChange={(e) => setExtraUrlsText(e.target.value)}
              />
              <p className="mt-1 text-xs text-muted">{t('settings.extraBaseUrlsHint')}</p>
            </div>
            <div className="sm:col-span-2">
              <Label htmlFor='cfg-reserved'>{t('settings.reservedCodes')}</Label>
              <Textarea id='cfg-reserved' rows={3} value={reservedText} onChange={(e) => setReservedText(e.target.value)} />
              <p className="mt-1 text-xs text-muted">{t('settings.reservedCodesHint')}</p>
            </div>
            <div>
              <Label htmlFor='cfg-login-attempts'>{t('settings.loginRateAttempts')}</Label>
              <Input
                id='cfg-login-attempts'
                type="number"
                min={0}
                value={form.login_rate_max_attempts}
                onChange={(e) => set('login_rate_max_attempts', Number(e.target.value))}
              />
              <p className="mt-1 text-xs text-muted">{t('settings.loginRateHint')}</p>
            </div>
            <div>
              <Label htmlFor='cfg-login-lock'>{t('settings.loginRateLock')}</Label>
              <Input
                id='cfg-login-lock'
                type="number"
                min={0}
                value={form.login_rate_lock_seconds}
                onChange={(e) => set('login_rate_lock_seconds', Number(e.target.value))}
              />
              <p className="mt-1 text-xs text-muted">{t('settings.loginRateLockHint')}</p>
            </div>
            <div>
              <Label htmlFor='cfg-session-ttl'>{t('settings.sessionTTL')}</Label>
              <Input
                id='cfg-session-ttl'
                type="number"
                min={0}
                value={form.session_ttl_minutes}
                onChange={(e) => set('session_ttl_minutes', Number(e.target.value))}
              />
              <p className="mt-1 text-xs text-muted">{t('settings.sessionTTLHint')}</p>
            </div>
            <div>
              <Label htmlFor='cfg-link-rate'>{t('settings.linkRate')}</Label>
              <Input
                id='cfg-link-rate'
                type="number"
                min={0}
                value={form.link_rate_per_second}
                onChange={(e) => set('link_rate_per_second', Number(e.target.value))}
              />
              <p className="mt-1 text-xs text-muted">{t('settings.linkRateHint')}</p>
            </div>
            <div>
              <Label>{t('settings.logLevel')}</Label>
              <Select
                value={form.log_level}
                onChange={(v) => set('log_level', v)}
                ariaLabel={t('settings.logLevel')}
                options={[
                  { value: 'debug', label: t('settings.logLevelDebug') },
                  { value: 'info', label: t('settings.logLevelInfo') },
                  { value: 'warning', label: t('settings.logLevelWarning') },
                  { value: 'error', label: t('settings.logLevelError') },
                ]}
              />
              <p className="mt-1 text-xs text-muted">{t('settings.logLevelHint')}</p>
            </div>
          </div>
        </Card>

        {/* Icon */}
        <Card className="p-6">
          <h2 className="mb-4 text-sm font-medium text-muted">{t('settings.icon')}</h2>
          <div className="flex flex-wrap items-center gap-3">
            <Button variant="outline" onClick={() => document.getElementById('icon-file')?.click()}>
              <Upload size={15} />
              {t('settings.uploadIcon')}
            </Button>
            <input
              id="icon-file"
              type="file"
              accept=".svg,.png"
              className="hidden"
              onChange={(e) => iconInput(e.target.files?.[0])}
            />
            {form.icon && (
              <Button variant="danger" onClick={async () => {
                try {
                  const res = await api.deleteIcon()
                  setForm((f) => (f ? { ...f, icon: res.icon } : f))
                  toast(t('settings.saved'))
                  queryClient.invalidateQueries({ queryKey: ['config'] })
                } catch (err) {
                  toast(err instanceof ApiError ? err.message : t('settings.saveFailed'), 'error')
                }
              }}>
                {t('settings.removeIcon')}
              </Button>
            )}
            <span className="short-code text-xs text-muted">{form.icon || 'default'}</span>
          </div>
        </Card>

        {/* UA blocks: like extra base urls and reserved codes, the list is
            edited here and applied by the save button */}
        <Card className="p-6">
          <h2 className="mb-1 text-sm font-medium text-muted">{t('settings.uaBlocks')}</h2>
          <p className="mb-4 text-xs text-muted">{t('settings.uaBlockHint')}</p>
          {/* Unconditional: block rules never expire, whether any exist yet or not. */}
          <p className="mb-4 text-xs text-muted">{t('common.never')}</p>
          <Textarea
            rows={3}
            value={uaText}
            onChange={(e) => setUaText(e.target.value)}
            placeholder={t('settings.uaPlaceholder')}
            aria-label={t('settings.uaPatterns')}
          />
        </Card>

        {/* IP blocks: banned addresses are refused on every route, session,
            API and health included */}
        <Card className="p-6">
          <h2 className="mb-1 text-sm font-medium text-muted">{t('settings.ipBlocks')}</h2>
          <p className="mb-4 text-xs text-muted">{t('settings.ipBlockHint')}</p>
          <Textarea
            rows={3}
            value={ipText}
            onChange={(e) => setIpText(e.target.value)}
            placeholder={t('settings.ipPlaceholder')}
            aria-label={t('settings.ipPatterns')}
          />
        </Card>

        {/* API tokens */}
        <TokenSection
          tokenNote={tokenNote}
          setTokenNote={setTokenNote}
          newToken={newToken}
          create={async () => {
            try {
              const res = await api.createToken(tokenNote)
              setNewToken(res.token)
              setTokenNote('')
              queryClient.invalidateQueries({ queryKey: ['tokens'] })
            } catch (err) {
              toast(err instanceof ApiError ? err.message : t('common.error'), 'error')
            }
          }}
        />

        <div className="flex justify-end">
          <Button onClick={save} disabled={saveMutation.isPending} className="px-8">
            {t('settings.save')}
          </Button>
        </div>
      </div>
    </div>
  )
}

type ConnStatus = 'checking' | 'connected' | 'unauthorized' | 'unreachable'

/** App-only card: the stored server, its live connection status and the
 *  disconnect action. Rendered above the config-dependent form so it always
 *  shows — even when the config request is slow or fails entirely. */
function ConnectionCard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const server = getServerConfig()
  const [confirmDisconnect, setConfirmDisconnect] = useState(false)
  const [status, setStatus] = useState<ConnStatus>('checking')

  // Probe the remote server through the authenticated config endpoint: only
  // a valid bearer token passes requireAuth, so a 401 means the stored token
  // is dead and a network error means the server is unreachable. Re-check on
  // mount, on app foreground and every 10s while the page stays open.
  useEffect(() => {
    let alive = true
    const check = async () => {
      try {
        await api.getConfig()
        if (alive) setStatus('connected')
      } catch (err) {
        if (!alive) return
        setStatus(err instanceof ApiError && err.status === 401 ? 'unauthorized' : 'unreachable')
      }
    }
    check()
    const timer = setInterval(check, 10_000)
    // Same pattern as App.tsx: grab the plugin off the runtime stub — the web
    // build must not depend on the @capacitor/app package.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cap = (window as any).Capacitor
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const appPlugin = cap?.Plugins?.App as any
    const handler = appPlugin?.addListener?.(
      'appStateChange',
      (s: { isActive: boolean }) => {
        if (s.isActive) check()
      },
    )
    return () => {
      alive = false
      clearInterval(timer)
      handler?.remove()
    }
  }, [])

  let dot = 'bg-current'
  let text = 'text-muted'
  let label = t('settings.connChecking')
  if (status === 'connected') {
    dot = 'bg-emerald-500'
    text = 'text-emerald-600 dark:text-emerald-400'
    label = t('settings.connConnected')
  } else if (status === 'unauthorized') {
    dot = 'bg-red-500'
    text = 'text-red-600 dark:text-red-400'
    label = t('settings.connUnauthorized')
  } else if (status === 'unreachable') {
    dot = 'bg-red-500'
    text = 'text-red-600 dark:text-red-400'
    label = t('settings.connUnreachable')
  }

  return (
    <>
      <Card className="p-6">
        <h2 className="mb-2 text-sm font-medium text-muted">{t('settings.appConnection')}</h2>
        <p className="mb-3 text-sm text-muted">
          {t('settings.connectedTo')} <span className="short-code">{server?.url ?? '—'}</span>
        </p>
        <div className="mb-3">
          <span className={`inline-flex items-center gap-1.5 text-sm ${text}`}>
            <span className={`size-1.5 rounded-full ${dot}`} />
            {label}
          </span>
        </div>
        <Button variant="outline" onClick={() => setConfirmDisconnect(true)}>
          <PlugZap size={15} />
          {t('settings.disconnectApp')}
        </Button>
      </Card>
      {/* The dialog is a sibling of the card, never inside it: the card's
          backdrop-blur creates a containing block that traps the dialog's
          fixed-positioned overlay inside the card area (the header got
          clipped). Page-level rendering keeps it like every other dialog. */}
      {/* Disconnecting drops the stored {url, token} — confirm first. */}
      <Dialog
        open={confirmDisconnect}
        onClose={() => setConfirmDisconnect(false)}
        title={t('settings.disconnectApp')}
      >
        <p className="text-sm text-muted">{t('settings.disconnectConfirm')}</p>
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" onClick={() => setConfirmDisconnect(false)}>
            {t('form.cancel')}
          </Button>
          <Button
            variant="danger"
            onClick={() => {
              setConfirmDisconnect(false)
              setServerConfig(null)
              navigate('/admin/connect', { replace: true })
            }}
          >
            {t('settings.disconnectApp')}
          </Button>
        </div>
      </Dialog>
    </>
  )
}

function TokenSection({
  tokenNote,
  setTokenNote,
  newToken,
  create,
}: {
  tokenNote: string
  setTokenNote: (v: string) => void
  newToken: string
  create: () => void
}) {
  const { t } = useTranslation()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const { data } = useQuery({ queryKey: ['tokens'], queryFn: api.tokens })
  const [revoking, setRevoking] = useState<TokenInfo | null>(null)
  const revokeMutation = useMutation({
    mutationFn: (id: number) => api.deleteToken(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tokens'] })
      setRevoking(null)
    },
    onError: (err: unknown) =>
      toast(err instanceof ApiError ? err.message : t('common.error'), 'error'),
  })
  // In the app, /docs/ lives on the remote server (not the WebView origin) —
  // link straight to it and hand it to the system browser via _system.
  const appMode = isApp()
  const server = getServerConfig()
  const docsUrl = appMode && server ? `${server.url.replace(/\/+$/, '')}/docs/` : '/docs/'

  return (
    <>
    <Card className="p-6">
      <h2 className="mb-1 text-sm font-medium text-muted">{t('settings.tokens')}</h2>
      <p className="mb-4 text-xs text-muted">{t('settings.tokensHint')}</p>
      {/* Unconditional: tokens never expire, whether any exist yet or not. */}
      <p className="mb-4 text-xs text-muted">{t('settings.tokenNeverExpires')}</p>
      <p className="mb-4">
        <a
          href={docsUrl}
          target={appMode ? '_system' : '_blank'}
          rel="noreferrer"
          className="text-sm font-medium text-accent-deep transition-colors hover:text-accent dark:text-accent"
        >
          {t('settings.apiDocs')} →
        </a>
      </p>
      {newToken && (
        <div className="mb-4 rounded-xl border border-accent/40 bg-accent-soft p-3">
          <p className="short-code break-all text-sm">{newToken}</p>
          <Button variant="ghost" className="mt-1 !p-1 text-xs" onClick={async () => {
            // Same multi-tier fallback chain as the link-row copy button.
            const ok = await copyText(newToken)
            toast(ok ? t('links.copied') : newToken, ok ? 'success' : 'error')
          }}>
            {t('links.copy')}
          </Button>
        </div>
      )}
      <div className="flex flex-col gap-2">
        {data?.tokens.map((tok) => (
          <div key={tok.id} className="flex items-center justify-between rounded-xl border border-hairline px-3.5 py-2">
            <div className="min-w-0">
              <span className="short-code text-sm">{tok.token}</span>
              {tok.note && <span className="ml-2 text-xs text-muted">{tok.note}</span>}
            </div>
            <Button variant="ghost" className="!p-1.5" onClick={() => setRevoking(tok)} aria-label={t('settings.revoke')}>
              <Trash2 size={15} />
            </Button>
          </div>
        ))}
        {data?.tokens.length === 0 && (
          <p className="py-2 text-sm text-muted">
            <KeyRound size={14} className="mr-1 inline" />
            {t('common.never')}
          </p>
        )}
        <div className="flex gap-2">
          <Input value={tokenNote} onChange={(e) => setTokenNote(e.target.value)} placeholder={t('form.note')} />
          <Button variant="outline" onClick={create} aria-label={t('settings.createToken')} className="!px-2.5">
            <Plus size={16} />
          </Button>
        </div>
      </div>
    </Card>
    {/* The dialog is a sibling of the glass card, never inside it — the card's
        backdrop-blur creates a containing block that traps fixed-positioned
        dialogs inside the card area (see the disconnect dialog note). */}
    <Dialog
      open={revoking !== null}
      onClose={() => setRevoking(null)}
      title={t('settings.revoke')}
    >
      <p className="text-sm text-muted">{t('settings.tokenRevokeConfirm')}</p>
      {revoking && <p className="short-code mt-2 text-sm font-medium">{revoking.token}</p>}
      <div className="mt-5 flex justify-end gap-2">
        <Button variant="ghost" onClick={() => setRevoking(null)}>
          {t('common.cancel')}
        </Button>
        <Button
          variant="danger"
          disabled={revokeMutation.isPending}
          onClick={() => revoking && revokeMutation.mutate(revoking.id)}
        >
          {t('common.delete')}
        </Button>
      </div>
    </Dialog>
    </>
  )
}
