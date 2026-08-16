import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { FileJson, FileSpreadsheet } from 'lucide-react'
import { api } from '../lib/api'
import { Button, Dialog, useToast } from './ui'

// Export dialog: CSV and JSON share one button; the format is picked here.
// Both exports carry the same 7 fields (code, url, title, description,
// expires_at, click_count, created_at).
export default function ExportDialog({
  open,
  onClose,
}: {
  open: boolean
  onClose: () => void
}) {
  const { t } = useTranslation()
  const { toast } = useToast()
  const [busy, setBusy] = useState(false)

  const download = (filename: string, blob: Blob) => {
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = filename
    a.click()
    URL.revokeObjectURL(a.href)
  }

  const exportCsv = async () => {
    if (busy) return
    setBusy(true)
    try {
      const res = await fetch('/api/v1/export.csv', { credentials: 'same-origin' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const date = new Date().toISOString().slice(0, 10)
      download(`gourl-links-${date}.csv`, await res.blob())
      onClose()
    } catch {
      toast(t('common.error'), 'error')
    } finally {
      setBusy(false)
    }
  }

  const exportJson = async () => {
    if (busy) return
    setBusy(true)
    try {
      const links = await api.exportJson()
      const date = new Date().toISOString().slice(0, 10)
      download(`gourl-links-${date}.json`, new Blob([JSON.stringify(links, null, 2)]))
      onClose()
    } catch {
      toast(t('common.error'), 'error')
    } finally {
      setBusy(false)
    }
  }

  const optionClass =
    'flex flex-col items-center gap-1.5 rounded-xl border border-hairline px-4 py-5 text-sm font-medium transition-colors hover:bg-black/5 dark:hover:bg-white/10'

  return (
    <Dialog open={open} onClose={onClose} title={t('links.exportTitle')}>
      <div className="flex gap-3">
        <button className={optionClass} onClick={exportCsv} disabled={busy}>
          <FileSpreadsheet size={22} className="text-accent" />
          {t('links.exportCsv')}
        </button>
        <button className={optionClass} onClick={exportJson} disabled={busy}>
          <FileJson size={22} className="text-accent" />
          {t('links.exportJson')}
        </button>
      </div>
      <div className="mt-4 flex justify-end">
        <Button variant="ghost" onClick={onClose}>
          {t('form.cancel')}
        </Button>
      </div>
    </Dialog>
  )
}
