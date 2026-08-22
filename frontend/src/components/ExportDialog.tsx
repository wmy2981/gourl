import { useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { FileJson, FileSpreadsheet, FileText } from 'lucide-react'
import { api } from '../lib/api'
import { blobToBase64, exportFilename, saveDownload } from '../lib/download'
import { Button, Dialog, useToast } from './ui'

// Export dialog: CSV, JSON and Markdown share one button row; the format is
// picked here. All three carry the same 7 fields (code, url, title,
// description, expires_at, click_count, created_at) — CSV/JSON for scripts,
// markdown for reading. In the app the files land in Downloads/gourl/ and the
// path is toasted; the web browser downloads normally.
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

  // Shared download path: the caller fetches the payload and this wraps the
  // busy flag, save + toast + close and error surfacing.
  const withBusy = async (fetchPayload: () => Promise<{ data: string; base64: boolean; mime: string }>) => {
    if (busy) return
    setBusy(true)
    try {
      const { data, base64, mime } = await fetchPayload()
      const path = await saveDownload(exportFilename('links', mimeToExt(mime)), data, base64, mime)
      if (path) toast(t('links.downloadedTo', { path }))
      onClose()
    } catch (err) {
      // Surface the plugin/API error so native save failures are diagnosable.
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    } finally {
      setBusy(false)
    }
  }

  const exportCsv = () =>
    withBusy(async () => ({
      data: await blobToBase64(await api.exportCsv()),
      base64: true,
      mime: 'text/csv',
    }))

  const exportMarkdown = () =>
    withBusy(async () => ({
      data: await blobToBase64(await api.exportMarkdown()),
      base64: true,
      mime: 'text/markdown',
    }))

  const exportJson = () =>
    withBusy(async () => {
      const links = await api.exportJson()
      return { data: JSON.stringify(links, null, 2), base64: false, mime: 'application/json' }
    })

// mime type → export filename extension.
function mimeToExt(mime: string): 'csv' | 'md' | 'json' {
  switch (mime) {
    case 'text/csv':
      return 'csv'
    case 'text/markdown':
      return 'md'
    default:
      return 'json'
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
        <button className={optionClass} onClick={exportMarkdown} disabled={busy}>
          <FileText size={22} className="text-accent" />
          {t('links.exportMarkdown')}
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
