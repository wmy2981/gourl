import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ArrowRight, KeyRound } from 'lucide-react'
import { api, ApiError } from '../lib/api'
import { Button, Input, Label, useToast } from '../components/ui'

export default function Login() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!password || busy) return
    setBusy(true)
    try {
      await api.login(password)
      navigate('/admin', { replace: true })
    } catch (err) {
      toast(
        err instanceof ApiError && err.code === 'auth_disabled'
          ? t('login.authDisabled')
          : t('login.wrongPassword'),
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
            <KeyRound className="text-accent-deep dark:text-accent" size={28} />
          </div>
          <h1 className="text-xl font-semibold">{t('login.heading')}</h1>
          <p className="mt-1 text-sm text-muted">
            {__APP_NAME__} <span className="short-code">v{__APP_VERSION__}</span>
          </p>
        </div>

        <Label htmlFor="password">{t('login.password')}</Label>
        <Input
          id="password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoFocus
          autoComplete="current-password"
        />

        <Button type="submit" disabled={busy || !password} className="mt-5 w-full">
          {t('login.signIn')}
          <ArrowRight size={16} />
        </Button>
      </form>
    </div>
  )
}
