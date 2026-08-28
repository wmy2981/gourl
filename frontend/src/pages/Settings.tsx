import { useCallback, useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Check, Copy, KeyRound, PlugZap, Plus, RefreshCw, Trash2, Upload } from 'lucide-react'
import { api, ApiError, getServerConfig, isApp, setServerConfig, type AppConfig, type TokenInfo } from '../lib/api'
import { apiErrorMessage } from '../lib/apiError'
import { hapticsEnabled, noSelectEnabled, setHapticsEnabled, setNoSelect } from '../lib/appSettings'
import { copyText } from '../lib/clipboard'
import { Button, Card, Dialog, Input, Label, Select, Switch, Textarea, useToast } from '../components/ui'

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
  // Autosave in-flight bookkeeping. Must live above the early return below:
  // hooks cannot be conditional (React #310).
  const savingRef = useRef(false)
  const pendingRef = useRef(false)

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
      toast(err instanceof ApiError ? apiErrorMessage(err, t) : t('settings.saveFailed'), 'error'),
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

  // Autosave: switches/selects save on change; text inputs and textareas
  // save on blur. While a request is in flight the latest pending save is
  // coalesced and re-fired when it completes.
  const save = (override?: AppConfig) => {
    if (!form) return
    if (savingRef.current) {
      pendingRef.current = true
      return
    }
    savingRef.current = true
    // `override` carries the not-yet-committed form state for immediate
    // controls: the setState closure still sees the old values when the
    // request body is built.
    const f = override ?? form
    saveMutation.mutate(
      {
        site: f.site,
        short_code_length: f.short_code_length,
        base_url: f.base_url,
        login_rate_max_attempts: f.login_rate_max_attempts,
        login_rate_lock_seconds: f.login_rate_lock_seconds,
        session_ttl_minutes: f.session_ttl_minutes,
        link_rate_per_second: f.link_rate_per_second,
        log_level: f.log_level,
        hard_delete: f.hard_delete,
        backup_on_edit: f.backup_on_edit,
        icon: f.icon,
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
      },
      {
        onSettled: () => {
          savingRef.current = false
          if (pendingRef.current) {
            pendingRef.current = false
            save()
          }
        },
      },
    )
  }
  // Immediate controls (switches, selects): apply then save with the next
  // state inline — setForm is async, so a plain read here would send the
  // pre-change values.
  const setAndSave = <K extends keyof AppConfig>(key: K, value: AppConfig[K]) => {
    const next = { ...form, [key]: value }
    setForm(next)
    save(next)
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
      toast(err instanceof ApiError ? apiErrorMessage(err, t) : t('settings.saveFailed'), 'error')
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
              <Input id='cfg-site-name' value={form.site.name} onChange={(e) => setSite('name', e.target.value)} onBlur={() => save()} />
            </div>
            <div>
              <Label htmlFor='cfg-site-title'>{t('settings.siteTitle')}</Label>
              <Input id='cfg-site-title' value={form.site.title} onChange={(e) => setSite('title', e.target.value)} onBlur={() => save()} />
            </div>
            <div className="sm:col-span-2">
              <Label htmlFor='cfg-description'>{t('settings.description')}</Label>
              <Textarea id='cfg-description' rows={3} value={form.site.description} onChange={(e) => setSite('description', e.target.value)} onBlur={() => save()} />
            </div>
          </div>
        </Card>

        {/* Security: password, session lifetime and the login rate limit —
            everything that gates admin access lives here */}
        <Card className="p-6">
          <h2 className="mb-2 text-sm font-medium text-muted">{t('settings.security')}</h2>
          <p className="mb-3 text-sm text-muted">{t('settings.securityHint')}</p>
          <Button variant="outline" onClick={() => navigate('/admin/change-password')}>
            <KeyRound size={15} />
            {t('settings.changePassword')}
          </Button>
          <div className="mt-4 grid grid-cols-1 gap-4 border-t border-hairline pt-4 sm:grid-cols-2">
            <div>
              <Label htmlFor='cfg-session-ttl'>{t('settings.sessionTTL')}</Label>
              <Input
                id='cfg-session-ttl'
                type="number"
                min={0}
                value={form.session_ttl_minutes}
                onChange={(e) => set('session_ttl_minutes', Number(e.target.value))}
                onBlur={() => save()}
              />
              <p className="mt-1 text-xs text-muted">{t('settings.sessionTTLHint')}</p>
            </div>
            <div>
              <Label htmlFor='cfg-login-attempts'>{t('settings.loginRateAttempts')}</Label>
              <Input
                id='cfg-login-attempts'
                type="number"
                min={0}
                value={form.login_rate_max_attempts}
                onChange={(e) => set('login_rate_max_attempts', Number(e.target.value))}
                onBlur={() => save()}
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
                onBlur={() => save()}
              />
              <p className="mt-1 text-xs text-muted">{t('settings.loginRateLockHint')}</p>
            </div>
          </div>
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
                min={2}
                max={64}
                value={form.short_code_length}
                onChange={(e) => set('short_code_length', Number(e.target.value))}
                onBlur={() => save()}
              />
            </div>
            <div>
              <Label htmlFor='cfg-base-url'>{t('settings.baseUrl')}</Label>
              <Input id='cfg-base-url' value={form.base_url} onChange={(e) => set('base_url', e.target.value)} onBlur={() => save()} placeholder="https://s.example.com" />
              <p className="mt-1 text-xs text-muted">{t('settings.baseUrlHint')}</p>
            </div>
            <div className="sm:col-span-2">
              <Label htmlFor="cfg-extra-urls">{t('settings.extraBaseUrls')}</Label>
              <Textarea
                id="cfg-extra-urls"
                rows={3}
                value={extraUrlsText}
                onChange={(e) => setExtraUrlsText(e.target.value)}
                onBlur={() => save()}
              />
              <p className="mt-1 text-xs text-muted">{t('settings.extraBaseUrlsHint')}</p>
            </div>
            <div className="sm:col-span-2">
              <Label htmlFor='cfg-reserved'>{t('settings.reservedCodes')}</Label>
              <Textarea id='cfg-reserved' rows={3} value={reservedText} onChange={(e) => setReservedText(e.target.value)} onBlur={() => save()} />
              <p className="mt-1 text-xs text-muted">{t('settings.reservedCodesHint')}</p>
            </div>
            <div>
              <Label htmlFor='cfg-link-rate'>{t('settings.linkRate')}</Label>
              <Input
                id='cfg-link-rate'
                type="number"
                min={0}
                value={form.link_rate_per_second}
                onChange={(e) => set('link_rate_per_second', Number(e.target.value))}
                onBlur={() => save()}
              />
              <p className="mt-1 text-xs text-muted">{t('settings.linkRateHint')}</p>
            </div>
            <div>
              <Label>{t('settings.logLevel')}</Label>
              <Select
                value={form.log_level}
                onChange={(v) => setAndSave('log_level', v)}
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
                  toast(err instanceof ApiError ? apiErrorMessage(err, t) : t('settings.saveFailed'), 'error')
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
            onBlur={() => save()}
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
            onBlur={() => save()}
            placeholder={t('settings.ipPlaceholder')}
            aria-label={t('settings.ipPatterns')}
          />
        </Card>

        {/* Data management: deletion mode and edit snapshots, both applied by
            the save button like every other config field */}
        <Card className="p-6">
          <h2 className="mb-4 text-sm font-medium text-muted">{t('settings.dataManagement')}</h2>
          <div className="flex flex-col gap-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <p className="text-sm">{t('settings.hardDelete')}</p>
                <p className="mt-0.5 text-xs text-muted">{t('settings.hardDeleteHint')}</p>
              </div>
              <Switch
                checked={form.hard_delete}
                onChange={(v) => setAndSave('hard_delete', v)}
                aria-label={t('settings.hardDelete')}
              />
            </div>
            <div className="flex items-start justify-between gap-3 border-t border-hairline pt-4">
              <div>
                <p className="text-sm">{t('settings.backupOnEdit')}</p>
                <p className="mt-0.5 text-xs text-muted">{t('settings.backupOnEditHint')}</p>
              </div>
              <Switch
                checked={form.backup_on_edit}
                onChange={(v) => setAndSave('backup_on_edit', v)}
                aria-label={t('settings.backupOnEdit')}
              />
            </div>
          </div>
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
              toast(t('settings.tokenCreated'))
            } catch (err) {
              toast(err instanceof ApiError ? apiErrorMessage(err, t) : t('common.error'), 'error')
            }
          }}
        />
      </div>
    </div>
  )
}

type ConnStatus = 'checking' | 'connected' | 'unauthorized' | 'unreachable'

// A probe that gets no answer in 5s counts as unreachable (the fetch abort
// lands in the catch as a non-ApiError, i.e. the unreachable branch).
const PROBE_TIMEOUT_MS = 5_000

/** App-only card: the stored server, its live connection status, server and
 *  app versions, a manual refresh and the disconnect action. Rendered above
 *  the config-dependent form so it always shows — even when the config
 *  request is slow or fails entirely. Exported for the component test. */
export function ConnectionCard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const server = getServerConfig()
  const [confirmDisconnect, setConfirmDisconnect] = useState(false)
  const [status, setStatus] = useState<ConnStatus>('checking')
  const [serverVersion, setServerVersion] = useState<string | null>(null)
  // Device-local app settings mirror the persisted state in React so the
  // switch reflects toggles; the storage write applies the effect live.
  const [noSelect, setNoSelectChecked] = useState(noSelectEnabled())
  // Haptics default to on (unlike noSelect); the storage write gates the
  // bridge calls at the source, so the switch takes effect immediately.
  const [haptics, setHaptics] = useState(hapticsEnabled())
  // Alive flag outside the effects so the refresh button can probe on demand
  // without restarting the mount effect's lifecycle.
  const aliveRef = useRef(true)

  // Probe the remote server through the auth/status endpoint: its
  // authenticated field is the real credential check, so connected vs
  // unauthorized comes from the body rather than a 401. Network errors and
  // timeouts still mean unreachable (5s abort, same as before). Older
  // servers without the authenticated field fall back to the config
  // endpoint's 401-based judgment. The public health endpoint rides along
  // for the server version; any failure leaves it unset (shown as "—").
  const check = useCallback(async () => {
    const [st, h] = await Promise.allSettled([
      api.authStatus({ signal: AbortSignal.timeout(PROBE_TIMEOUT_MS) }),
      api.health({ signal: AbortSignal.timeout(PROBE_TIMEOUT_MS) }),
    ])
    if (!aliveRef.current) return
    if (st.status === 'fulfilled') {
      const authed = (st.value as { authenticated?: boolean }).authenticated
      if (authed === undefined) {
        // Legacy server: re-judge through the config probe.
        try {
          await api.getConfig({ signal: AbortSignal.timeout(PROBE_TIMEOUT_MS) })
          setStatus('connected')
        } catch (err) {
          setStatus(err instanceof ApiError && err.status === 401 ? 'unauthorized' : 'unreachable')
        }
      } else {
        setStatus(authed ? 'connected' : 'unauthorized')
      }
    } else {
      // A rejected probe: a 401 means the server answered and refused the
      // token, anything else (network error, 5s abort) means unreachable.
      const err = st.reason
      setStatus(err instanceof ApiError && err.status === 401 ? 'unauthorized' : 'unreachable')
    }
    setServerVersion(h.status === 'fulfilled' ? h.value.version : null)
  }, [])

  // Re-check on mount, on app foreground and every 10s while the page stays
  // open; the refresh button triggers the same probe on demand.
  useEffect(() => {
    aliveRef.current = true
    void check()
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
        if (s.isActive) void check()
      },
    )
    return () => {
      aliveRef.current = false
      clearInterval(timer)
      handler?.remove()
    }
  }, [check])

  const refresh = () => {
    if (status === 'checking') return
    // Back to the probing state so the dot and the spin give immediate
    // feedback while the fresh probe is in flight.
    setStatus('checking')
    void check()
  }

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
        {/* Versions under the address: the server one comes from the last
            probe's health call (— when unavailable), the app one is the
            embedded build constant. */}
        <div className="mb-3 space-y-0.5 text-xs text-muted">
          <p>
            {t('settings.serverVersion')}{' '}
            <span className="short-code">{serverVersion != null ? `v${serverVersion}` : '—'}</span>
          </p>
          <p>
            {t('settings.appVersion')} <span className="short-code">v{__APP_VERSION__}</span>
          </p>
        </div>
        <div className="mb-3 pl-2">
          <span className={`inline-flex items-center gap-1.5 text-sm ${text}`}>
            <span className={`size-1.5 rounded-full ${dot}`} />
            {label}
          </span>
        </div>
        <div className="flex gap-2">
          {/* Same icon-only refresh as the Links page header; spinning while
              a probe is in flight and disabled during it. */}
          <Button
            variant="outline"
            onClick={refresh}
            disabled={status === 'checking'}
            aria-label={t('settings.connRefresh')}
            className="!px-2.5"
          >
            <RefreshCw size={16} className={status === 'checking' ? 'animate-spin' : ''} />
          </Button>
          <Button variant="outline" onClick={() => setConfirmDisconnect(true)}>
            <PlugZap size={15} />
            {t('settings.disconnectApp')}
          </Button>
        </div>
        {/* Device-local app settings: stored on the device, applied live. */}
        <div className="mt-4 flex items-center justify-between gap-3 border-t border-hairline pt-4">
          <span className="text-sm">{t('settings.noTextSelect')}</span>
          <Switch
            checked={noSelect}
            onChange={(v) => {
              setNoSelectChecked(v)
              setNoSelect(v)
            }}
            aria-label={t('settings.noTextSelect')}
          />
        </div>
        <div className="mt-4 flex items-center justify-between gap-3">
          <span className="text-sm">{t('settings.hapticsFeedback')}</span>
          <Switch
            checked={haptics}
            onChange={(v) => {
              setHaptics(v)
              setHapticsEnabled(v)
            }}
            aria-label={t('settings.hapticsFeedback')}
          />
        </div>
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

/** The API tokens card: list, create and delete. Exported for the component
 *  test — the parent only wires the token-note state. */
export function TokenSection({
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
  const [tokenCopied, setTokenCopied] = useState(false)
  const revokeMutation = useMutation({
    mutationFn: (id: number) => api.deleteToken(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tokens'] })
      setRevoking(null)
      toast(t('settings.tokenRevoked'))
    },
    onError: (err: unknown) =>
      toast(err instanceof ApiError ? apiErrorMessage(err, t) : t('common.error'), 'error'),
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
        <div className="mb-4 flex items-center justify-between gap-2 rounded-xl border border-accent/40 bg-accent-soft p-3">
          <p className="short-code break-all text-sm">{newToken}</p>
          {/* Same icon treatment as the link-row copy button. */}
          <button
            onClick={async () => {
              // Same multi-tier fallback chain as the link-row copy button.
              const ok = await copyText(newToken)
              if (ok) {
                setTokenCopied(true)
                setTimeout(() => setTokenCopied(false), 1500)
              }
              toast(ok ? t('links.copied') : t('links.copyFailed'), ok ? 'success' : 'error')
            }}
            title={t('links.copy')}
            className="shrink-0 rounded-md p-1 text-muted transition-colors hover:bg-accent-soft hover:text-accent-deep dark:hover:text-accent"
          >
            {tokenCopied ? <Check size={15} className="text-success" /> : <Copy size={15} />}
          </button>
        </div>
      )}
      <div className="flex flex-col gap-2">
        {data?.tokens.map((tok) => (
          <div key={tok.id} className="flex items-center justify-between rounded-xl border border-hairline px-3.5 py-2">
            <div className="min-w-0">
              {/* The API only ever returns the 8-char prefix; render it as a
                  truncated preview, never as if it were the full token. */}
              <span className="short-code text-sm">{tok.token}…</span>
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
      {revoking && (
        <div className="mt-2">
          <p className="short-code text-sm font-medium">{revoking.token}…</p>
          {revoking.note && <p className="mt-1 text-xs text-muted">{revoking.note}</p>}
        </div>
      )}
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
