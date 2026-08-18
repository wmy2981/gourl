import { useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { FileJson, FileSpreadsheet } from 'lucide-react'
import { api } from '../lib/api'
import { blobToBase64, saveDownload } from '../lib/download'
import { Button, Dialog, useToast } from './ui'

// Export dialog: CSV and JSON share one button; the format is picked here.
// Both exports carry the same 7 fields (code, url, title, description,
// expires_at, click_count, created_at). In the app the files land in
// Downloads/gourl/ and the path is toasted; the web browser downloads normally.
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

  const exportCsv = async () => {
    if (busy) return
    setBusy(true)
    try {
      const blob = await api.exportCsv()
      const date = new Date().toISOString().slice(0, 10)
      const path = await saveDownload(`gourl-links-${date}.csv`, await blobToBase64(blob), true, 'text/csv')
      if (path) toast(t('links.downloadedTo', { path }))
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
      const path = await saveDownload(
        `gourl-links-${date}.json`,
        JSON.stringify(links, null, 2),
        false,
        'application/json',
      )
      if (path) toast(t('links.downloadedTo', { path }))
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
      <p className="mt-3 text-center text-xs text-muted">
        <Trans i18nKey="links.exportHint">
          For database parsing, use the db-export script (
          <a
            href="https://raw.githubusercontent.com/wmy2981/gourl/main/scripts/db-export.mts"
            target="_blank"
            rel="noreferrer"
            className="text-accent underline-offset-2 hover:underline"
          >
            GitHub download
          </a>
          )
        </Trans>
      </p>
      <div className="mt-4 flex justify-end">
        <Button variant="ghost" onClick={onClose}>
          {t('form.cancel')}
        </Button>
      </div>
    </Dialog>
  )
}
