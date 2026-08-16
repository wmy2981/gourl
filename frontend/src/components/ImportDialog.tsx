import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Upload } from 'lucide-react'
import { api, ApiError, type ImportItem } from '../lib/api'
import { parseCSV } from '../lib/csv'
import { Button, Dialog, Label, Textarea, useToast } from './ui'

// Batch import: items pasted as JSON or loaded from a file, with a conflict
// policy for codes that already exist. JSON files are parsed as-is; CSV files
// expect a header row matching the export shape
// (code,url,title,description,expires_at,click_count,created_at).
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
  const [conflict, setConflict] = useState<'error' | 'skip' | 'update'>('error')
  const [busy, setBusy] = useState(false)

  const selectClass =
    'w-40 rounded-xl border border-hairline bg-white/70 dark:bg-white/[0.07] px-3 py-2 text-sm outline-none transition focus:border-accent focus:ring-2 focus:ring-accent/30'

  // Load a .json or .csv file and turn it into the editable JSON textarea.
  const loadFile = async (file: File | undefined) => {
    if (!file) return
    try {
      const raw = await file.text()
      if (file.name.toLowerCase().endsWith('.csv')) {
        const rows = parseCSV(raw)
        const items: ImportItem[] = rows.map((r) => {
          const item: ImportItem = { url: r.url }
          if (r.code) item.code = r.code
          if (r.title) item.title = r.title
          if (r.description) item.description = r.description
          // Number = unix seconds; anything else (a yyyy-MM-dd string) is
          // passed through — the backend accepts both.
          if (r.expires_at) item.expires_at = Number(r.expires_at) || r.expires_at
          if (r.click_count) item.click_count = Number(r.click_count)
          if (r.created_at) item.created_at = Number(r.created_at)
          return item
        })
        setText(JSON.stringify(items, null, 2))
      } else {
        // .json (or unknown): keep the text as-is; submit validates it.
        setText(raw)
      }
    } catch {
      toast(t('form.invalidCode'), 'error')
    }
  }

  const submit = async () => {
    if (busy) return
    setBusy(true)
    try {
      const parsed = JSON.parse(text)
      if (!Array.isArray(parsed)) throw new Error('not an array')
      const res = await api.batchCreate(parsed as ImportItem[], conflict)
      toast(t('form.importResults', { created: res.created, failed: res.failed }))
      setText('')
      onImported()
      onClose()
    } catch (err) {
      toast(err instanceof ApiError ? err.message : t('form.invalidCode'), 'error')
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
      <div className="mt-4 flex items-center justify-between gap-2">
        <div>
          <Label htmlFor="import-conflict">{t('form.importConflict')}</Label>
          <select
            id="import-conflict"
            value={conflict}
            onChange={(e) => setConflict(e.target.value as 'error' | 'skip' | 'update')}
            className={selectClass}
          >
            <option value="error">{t('form.conflictError')}</option>
            <option value="skip">{t('form.conflictSkip')}</option>
            <option value="update">{t('form.conflictUpdate')}</option>
          </select>
        </div>
        <div className="flex gap-2">
          <Button variant="ghost" onClick={onClose}>
            {t('form.cancel')}
          </Button>
          <Button onClick={submit} disabled={busy || !text.trim()}>
            {t('form.create')}
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
