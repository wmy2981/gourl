import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ArrowRight, ExternalLink, PlugZap } from 'lucide-react'
import { api, ApiError, setServerConfig } from '../lib/api'
import { Button, Input, Label, useToast } from '../components/ui'

// Mobile-app connect screen: point the app at a gourl server and store an
// API token from the web console (Settings → API tokens). Everything after
// this runs in token mode — login and setup pages never appear.
export default function Connect() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const navigate = useNavigate()
  const [url, setUrl] = useState('')
  const [token, setToken] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!url || !token || busy) return
    let parsed: URL
    try {
      parsed = new URL(url.trim())
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') throw new Error()
    } catch {
      toast(t('connect.invalidUrl'), 'error')
      return
    }
    setBusy(true)
    // Probe the server with the token before persisting anything; a bad
    // token or unreachable host rolls the config back.
    setServerConfig({ url: parsed.origin, token: token.trim() })
    try {
      await api.authStatus()
    } catch (err) {
      setServerConfig(null)
      toast(err instanceof ApiError ? err.message : t('connect.failed'), 'error')
      setBusy(false)
      return
    }
    setBusy(false)
    navigate('/admin', { replace: true })
  }

  const openServer = () => {
    // In the Capacitor app '_system' hands the URL to the default browser;
    // on the web it is a plain new tab.
    window.open(url.trim() || 'https://github.com/wmy2981/gourl', '_system')
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <form onSubmit={submit} className="glass w-full max-w-sm p-8">
        <div className="mb-6 text-center">
          <div className="mb-3 inline-block rounded-2xl bg-accent-soft p-3">
            <PlugZap className="text-accent-deep dark:text-accent" size={28} />
          </div>
          <h1 className="text-xl font-semibold">{t('connect.heading')}</h1>
          <p className="mt-1 text-sm text-muted">{t('connect.sub')}</p>
        </div>

        <Label htmlFor="server-url">{t('connect.url')}</Label>
        <Input
          id="server-url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="http://192.168.1.10:8081"
          autoFocus
          inputMode="url"
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
        />
        <p className="mt-1 text-xs text-muted">{t('connect.urlHint')}</p>

        <Label htmlFor="api-token">{t('connect.token')}</Label>
        <Input
          id="api-token"
          value={token}
          onChange={(e) => setToken(e.target.value)}
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
        />
        <p className="mt-1 text-xs text-muted">{t('connect.tokenHint')}</p>

        <Button type="submit" disabled={busy || !url || !token} className="mt-5 w-full">
          {t('connect.connect')}
          <ArrowRight size={16} />
        </Button>

        <div className="mt-5 rounded-xl bg-black/[0.03] p-4 text-xs leading-relaxed text-muted dark:bg-white/5">
          <ol className="list-decimal space-y-1 pl-4">
            <li>{t('connect.step1')}</li>
            <li>{t('connect.step2')}</li>
            <li>{t('connect.step3')}</li>
          </ol>
          <button
            type="button"
            onClick={openServer}
            className="mt-3 flex items-center gap-1.5 text-accent-deep transition-opacity hover:opacity-80 dark:text-accent"
          >
            <ExternalLink size={13} />
            {t('connect.openServer')}
          </button>
        </div>
      </form>
    </div>
  )
}
