import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api, ApiError, type Link } from '../lib/api'
import { Button, Dialog, Input, Label } from './ui'

export default function LinkFormDialog({
  link,
  open,
  onClose,
  onSaved,
}: {
  link: Link | null
  open: boolean
  onClose: () => void
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [url, setUrl] = useState('')
  const [code, setCode] = useState('')
  const [expiresAt, setExpiresAt] = useState('0')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (open) {
      setUrl(link?.url ?? '')
      setCode(link?.code ?? '')
      setExpiresAt(String(link?.expires_at ?? 0))
      setError('')
    }
  }, [open, link])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (busy) return
    setBusy(true)
    setError('')
    try {
      const expires = Number(expiresAt)
      if (link) {
        await api.updateLink(link.code, { url, code, expires_at: expires })
      } else {
        await api.createLink({ url, code: code || undefined, expires_at: expires })
      }
      onSaved()
      onClose()
    } catch (err) {
      if (err instanceof ApiError) {
        setError(
          err.code === 'code_taken'
            ? t('form.codeTaken')
            : err.code === 'reserved_code'
              ? t('form.reserved')
              : err.code === 'invalid_code'
                ? t('form.invalidCode')
                : err.code === 'invalid_request'
                  ? t('form.invalidUrl')
                  : err.message,
        )
      } else {
        setError(t('common.error'))
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onClose={onClose} title={link ? t('form.editTitle') : t('form.createTitle')}>
      <form onSubmit={submit} className="flex flex-col gap-4">
        <div>
          <Label>{t('form.url')}</Label>
          <Input value={url} onChange={(e) => setUrl(e.target.value)} placeholder={t('form.urlPlaceholder')} required />
        </div>
        <div>
          <Label>{t('form.code')}</Label>
          <Input
            value={code}
            onChange={(e) => setCode(e.target.value)}
            placeholder={t('form.codePlaceholder')}
            className="short-code"
          />
        </div>
        <div>
          <Label>{t('form.expiresAt')}</Label>
          <Input
            type="number"
            min={0}
            value={expiresAt}
            onChange={(e) => setExpiresAt(e.target.value)}
          />
        </div>
        {error && <p className="text-sm text-danger">{error}</p>}
        <div className="mt-1 flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t('form.cancel')}
          </Button>
          <Button type="submit" disabled={busy}>
            {t('form.save')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
