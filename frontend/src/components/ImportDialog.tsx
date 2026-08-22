import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { json } from '@codemirror/lang-json'
import { Upload } from 'lucide-react'
import { api, ApiError, type ImportItem } from '../lib/api'
import { parseCSV } from '../lib/csv'
import { jsonHighlight } from '../lib/jsonHighlight'
import { Button, Dialog, Label, Select, useToast } from './ui'
import CodeEditor from './CodeEditor'

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
          // click_count is deliberately dropped: imports never fabricate
          // click history.
          if (r.created_at) item.created_at = Number(r.created_at)
          return item
        })
        setText(JSON.stringify(items, null, 2))
      } else {
        // .json (or unknown): keep the text as-is; submit validates it.
        setText(raw)
      }
    } catch {
      toast(t('form.fileReadFailed'), 'error')
    }
  }

  const submit = async () => {
    if (busy) return
    setBusy(true)
    try {
      const parsed = JSON.parse(text)
      if (!Array.isArray(parsed)) throw new Error('not an array')
      const res = await api.batchCreate(parsed as ImportItem[], conflict)
      toast(
        t('form.importResults', {
          created: res.succeeded,
          skipped: res.skipped,
          updated: res.updated,
          failed: res.failed,
        }),
      )
      setText('')
      onImported()
      onClose()
    } catch (err) {
      toast(err instanceof ApiError ? err.message : t('form.importFailed'), 'error')
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
      <CodeEditor
        className="h-60"
        value={text}
        onChange={setText}
        placeholder='[{"url": "https://example.com/1"}, {"url": "https://example.com/2", "code": "two"}]'
        ariaLabel={t('form.importHint')}
        extensions={[json(), jsonHighlight]}
      />
      <div className="mt-4 flex items-end justify-between gap-2">
        <div className="w-44">
          <Label>{t('form.importConflict')}</Label>
          <Select
            value={conflict}
            onChange={(v) => setConflict(v as 'error' | 'skip' | 'update')}
            ariaLabel={t('form.importConflict')}
            options={[
              { value: 'error', label: t('form.conflictError') },
              { value: 'skip', label: t('form.conflictSkip') },
              { value: 'update', label: t('form.conflictUpdate') },
            ]}
          />
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
