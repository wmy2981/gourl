import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Upload } from 'lucide-react'
import { api, ApiError } from '../lib/api'
import { parseCSV } from '../lib/csv'
import { Button, Dialog, Textarea, useToast } from './ui'

// Batch import: a JSON array of {url, code?, expires_at?} items pasted or
// loaded from a file. JSON files are parsed as-is; CSV files expect a header
// row (code,url,title,description,expires_at) — the same shape the exports
// produce.
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
  const fileRef = useRef<HTMLInputElement>(null)
  const [text, setText] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  // Load a .json or .csv file and turn it into the editable JSON textarea.
  const loadFile = async (file: File | undefined) => {
    if (!file) return
    try {
      const raw = await file.text()
      if (file.name.toLowerCase().endsWith('.csv')) {
        const rows = parseCSV(raw)
        const items = rows.map((r) => {
          const item: Record<string, unknown> = { url: r.url }
          if (r.code) item.code = r.code
          if (r.title) item.title = r.title
          if (r.description) item.description = r.description
          if (r.expires_at) item.expires_at = Number(r.expires_at)
          return item
        })
        setText(JSON.stringify(items, null, 2))
      } else {
        // .json (or unknown): keep the text as-is; submit validates it.
        setText(raw)
      }
      setError('')
    } catch {
      setError(t('form.invalidCode'))
    }
  }

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
      <div className="mb-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-xs text-muted">{t('form.importHint')}</p>
          <Button variant="outline" className="shrink-0" onClick={() => fileRef.current?.click()}>
            <Upload size={14} />
            {t('form.importFromFile')}
          </Button>
        </div>
        <input
          ref={fileRef}
          type="file"
          accept=".json,.csv"
          className="hidden"
          onChange={(e) => loadFile(e.target.files?.[0])}
        />
      </div>
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
