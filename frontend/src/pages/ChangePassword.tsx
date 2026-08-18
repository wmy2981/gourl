import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, KeyRound } from 'lucide-react'
import { api, ApiError } from '../lib/api'
import { Button, Input, Label, useToast } from '../components/ui'

// Standalone page opened from the settings page in a new tab: changing the
// password bumps the session epoch, revoking every session — including this
// one — so the flow ends on the login page.
export default function ChangePassword() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const navigate = useNavigate()
  const { data: health } = useQuery({ queryKey: ['health'], queryFn: api.health })
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (busy) return
    if (!oldPassword || !newPassword || !confirm) return
    if (newPassword.length < 8) {
      toast(t('changePassword.tooShort'), 'error')
      return
    }
    if (newPassword !== confirm) {
      toast(t('changePassword.mismatch'), 'error')
      return
    }
    setBusy(true)
    try {
      await api.changePassword(oldPassword, newPassword)
      toast(t('changePassword.success'))
      // Every session — this one included — was revoked on purpose.
      navigate('/admin/login', { replace: true })
    } catch (err) {
      if (err instanceof ApiError && err.code === 'unauthorized') {
        toast(t('changePassword.wrongCurrent'), 'error')
      } else if (err instanceof ApiError && err.code === 'rate_limited') {
        toast(t('login.rateLimited'), 'error')
      } else {
        toast(err instanceof ApiError ? err.message : t('changePassword.failed'), 'error')
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <form onSubmit={submit} className="glass w-full max-w-sm p-8">
        {/* The page is opened from the settings page (new tab in the web
            console, same view in the app); back returns to it. */}
        <div className="mb-4 flex justify-start">
          <Button variant="ghost" className="!p-1.5" onClick={() => navigate('/admin/settings')}>
            <ArrowLeft size={16} />
            {t('common.back')}
          </Button>
        </div>
        <div className="mb-6 text-center">
          <div className="mb-3 inline-block rounded-2xl bg-accent-soft p-3">
            <KeyRound className="text-accent-deep dark:text-accent" size={28} />
          </div>
          <h1 className="text-xl font-semibold">{t('changePassword.heading')}</h1>
          <p className="mt-1 text-sm text-muted">
            {health?.name || __APP_NAME__} <span className="short-code">v{__APP_VERSION__}</span>
          </p>
        </div>

        <Label htmlFor="old-password">{t('changePassword.current')}</Label>
        <Input
          id="old-password"
          type="password"
          value={oldPassword}
          onChange={(e) => setOldPassword(e.target.value)}
          autoFocus
          autoComplete="current-password"
        />

        <div className="mt-4">
          <Label htmlFor="new-password">{t('changePassword.new')}</Label>
          <Input
            id="new-password"
            type="password"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            autoComplete="new-password"
          />
        </div>

        <div className="mt-4">
          <Label htmlFor="confirm-password">{t('changePassword.confirm')}</Label>
          <Input
            id="confirm-password"
            type="password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            autoComplete="new-password"
          />
        </div>

        <Button
          type="submit"
          disabled={busy || !oldPassword || !newPassword || !confirm}
          className="mt-5 w-full"
        >
          {t('changePassword.submit')}
        </Button>
      </form>
    </div>
  )
}
