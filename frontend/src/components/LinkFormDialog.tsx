import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api, ApiError, type Link } from '../lib/api'
import { Button, Dialog, Input, Label, Textarea, useToast } from './ui'
import DateInput from './DateInput'

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
  const { toast } = useToast()
  const [url, setUrl] = useState('')
  const [code, setCode] = useState('')
  const [description, setDescription] = useState('')
  // Expiry as a calendar date (yyyy-mm-dd); null while the typed text is not
  // a valid date; empty string means never expires.
  const [expiresDate, setExpiresDate] = useState<string | null>('')
  const [busy, setBusy] = useState(false)

  // unix seconds → local yyyy-mm-dd for the date input ('' when never).
  const toDate = (unix: number) => {
    if (!unix) return ''
    const d = new Date(unix * 1000)
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
  }
  // local yyyy-mm-dd → unix seconds at local midnight.
  const toUnix = (date: string) => {
    if (!date) return 0
    return Math.floor(new Date(`${date}T00:00:00`).getTime() / 1000)
  }

  useEffect(() => {
    if (open) {
      setUrl(link?.url ?? '')
      setCode(link?.code ?? '')
      setDescription(link?.description ?? '')
      setExpiresDate(toDate(link?.expires_at ?? 0))
    }
  }, [open, link])

  // Scheme shortcut buttons: an empty input gets the clicked scheme; an input
  // with the other http(s) scheme has its prefix swapped; the same scheme is
  // left alone ("don't repeat"). A scheme-less URL just gets the prefix.
  const applyScheme = (scheme: 'http' | 'https') => {
    const trimmed = url.trim()
    if (!trimmed) {
      setUrl(`${scheme}://`)
      return
    }
    const lower = trimmed.toLowerCase()
    if (lower.startsWith(`${scheme}://`)) return
    if (lower.startsWith('http://') || lower.startsWith('https://')) {
      setUrl(trimmed.replace(/^https?:\/\//i, `${scheme}://`))
      return
    }
    setUrl(`${scheme}://${trimmed}`)
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (busy) return
    if (!url.trim()) {
      toast(t('form.urlRequired'), 'error')
      return
    }
    if (expiresDate === null) {
      toast(t('form.invalidDate'), 'error')
      return
    }
    setBusy(true)
    try {
      const expires = toUnix(expiresDate)
      if (link) {
        await api.updateLink(link.code, { url, code, description, expires_at: expires })
      } else {
        await api.createLink({ url, code: code || undefined, description, expires_at: expires })
      }
      onSaved()
      onClose()
    } catch (err) {
      if (err instanceof ApiError) {
        toast(
          err.code === 'code_taken'
            ? t('form.codeTaken')
            : err.code === 'reserved_code'
              ? t('form.reserved')
              : err.code === 'invalid_code'
                ? t('form.invalidCode')
                : err.code === 'invalid_request'
                  ? t('form.invalidUrl')
                  : err.code === 'description_too_long'
                    ? t('form.descriptionTooLong')
                    : err.code === 'self_link_target'
                      ? t('form.selfLinkTarget')
                      : err.message,
          'error',
        )
      } else {
        toast(t('common.error'), 'error')
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onClose={onClose} title={link ? t('form.editTitle') : t('form.createTitle')}>
      <form onSubmit={submit} className="flex flex-col gap-4">
        <div>
          <Label htmlFor="link-url">{t('form.url')}</Label>
          <Input
            id="link-url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder={t('form.urlPlaceholder')}
          />
          <div className="mt-1.5 flex items-center gap-1.5">
            <span className="text-xs text-muted">{t('form.schemeHint')}</span>
            <button
              type="button"
              onClick={() => applyScheme('http')}
              className="rounded-md border border-hairline px-1.5 py-0.5 text-xs text-muted transition-colors hover:border-accent hover:text-accent-deep dark:hover:text-accent"
            >
              http://
            </button>
            <button
              type="button"
              onClick={() => applyScheme('https')}
              className="rounded-md border border-hairline px-1.5 py-0.5 text-xs text-muted transition-colors hover:border-accent hover:text-accent-deep dark:hover:text-accent"
            >
              https://
            </button>
          </div>
        </div>
        <div>
          <Label htmlFor="link-code">{t('form.code')}</Label>
          <Input
            id="link-code"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            placeholder={t('form.codePlaceholder')}
            className="short-code"
          />
        </div>
        <div>
          <Label htmlFor="link-description">{t('form.description')}</Label>
          <Textarea
            id="link-description"
            rows={2}
            maxLength={500}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder={t('form.descriptionPlaceholder')}
          />
        </div>
        <div>
          <Label htmlFor="link-expires">{t('form.expiresAt')}</Label>
          <DateInput
            id="link-expires"
            value={expiresDate ?? ''}
            onChange={setExpiresDate}
            ariaLabel={t('form.expiresAt')}
          />
        </div>
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
