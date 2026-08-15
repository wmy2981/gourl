import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api, ApiError } from '../lib/api'
import { Button, Dialog, Textarea, useToast } from './ui'

// Batch import: a JSON array of {url, code?, expires_at?} items, one per
// intent, e.g. [{"url": "https://a.com/x"}, {"url": "https://b.com/y", "code": "b"}]
export default function ImportDialog({
  open,
  onClose,
  onImported,
}: {
  open: boolean
  onClose: () => void
  onImported: () => void
}) {
  const { t } = useTranslation()
  const { toast } = useToast()
  const [text, setText] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    if (busy) return
    setBusy(true)
    setError('')
    try {
      const parsed = JSON.parse(text)
      if (!Array.isArray(parsed)) throw new Error('not an array')
      const res = await api.batchCreate(
        parsed.map((item: unknown) => item as { url: string; code?: string; expires_at?: number }),
      )
      toast(t('form.importResults', { created: res.created, failed: res.failed }))
      setText('')
      onImported()
      onClose()
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError(t('form.invalidCode'))
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onClose={onClose} title={t('form.importTitle')} wide>
      <p className="mb-3 text-xs text-muted">{t('form.importHint')}</p>
      <Textarea
        rows={10}
        value={text}
        onChange={(e) => setText(e.target.value)}
        className="short-code"
        placeholder='[{"url": "https://example.com/1"}, {"url": "https://example.com/2", "code": "two"}]'
      />
      {error && <p className="mt-3 text-sm text-danger">{error}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>
          {t('form.cancel')}
        </Button>
        <Button onClick={submit} disabled={busy || !text.trim()}>
          {t('form.create')}
        </Button>
      </div>
    </Dialog>
  )
}
