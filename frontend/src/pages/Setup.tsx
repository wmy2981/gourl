import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ArrowRight, ShieldCheck } from 'lucide-react'
import { api, ApiError } from '../lib/api'
import { Button, Input, Label, useToast } from '../components/ui'

// Setup runs once, while the server has no admin password yet: the first
// visitor picks one and is logged in immediately. It reuses the login page
// shell, differing only in copy, a confirm field, and the endpoint.
export default function Setup() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const navigate = useNavigate()
  // The configured service name replaces the gourl brand in the brand line;
  // it comes from the public health endpoint so it works before any admin
  // password exists (the config API refuses requests in setup mode).
  const { data: health } = useQuery({ queryKey: ['health'], queryFn: api.health })
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!password || busy) return
    if (password !== confirm) {
      toast(t('setup.mismatch'), 'error')
      return
    }
    setBusy(true)
    try {
      await api.setupAdmin(password)
      navigate('/admin', { replace: true })
    } catch (err) {
      toast(
        err instanceof ApiError && err.code === 'weak_password'
          ? t('setup.weak')
          : err instanceof ApiError
            ? err.message
            : t('common.error'),
        'error',
      )
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <form onSubmit={submit} className="glass w-full max-w-sm p-8">
        <div className="mb-6 text-center">
          <div className="mb-3 inline-block rounded-2xl bg-accent-soft p-3">
            <ShieldCheck className="text-accent-deep dark:text-accent" size={28} />
          </div>
          <h1 className="text-xl font-semibold">{t('setup.heading')}</h1>
          <p className="mt-1 text-sm text-muted">
            {health?.name || __APP_NAME__} <span className="short-code">v{__APP_VERSION__}</span>
          </p>
          <p className="mt-3 text-xs leading-relaxed text-muted">{t('setup.sub')}</p>
        </div>

        <Label htmlFor="new-password">{t('setup.password')}</Label>
        <Input
          id="new-password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoFocus
          autoComplete="new-password"
        />

        <div className="mt-4">
          <Label htmlFor="confirm-password">{t('setup.confirm')}</Label>
          <Input
            id="confirm-password"
            type="password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            autoComplete="new-password"
          />
        </div>

        <Button type="submit" disabled={busy || !password || !confirm} className="mt-5 w-full">
          {t('setup.setup')}
          <ArrowRight size={16} />
        </Button>
      </form>
    </div>
  )
}
