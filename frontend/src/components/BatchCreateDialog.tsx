import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api, ApiError, type ImportItem } from '../lib/api'
import { parseBatchLine, type ParsedBatchItem } from '../lib/batch'
import { Button, Dialog, Label, Textarea, useToast } from './ui'

interface Failure {
  line: string
  reason: string
}

// Batch create: one strict-syntax line per item
//   [code](date)url
// (both brackets optional). Format errors are flagged before submit; the
// server then creates each valid line independently, and the failed lines
// stay editable for a one-click retry.
export default function BatchCreateDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean
  onClose: () => void
  onCreated: () => void
}) {
  const { t } = useTranslation()
  const { toast } = useToast()
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  const [failures, setFailures] = useState<Failure[]>([])

  // Live per-line validation: valid items (skipping comments/blank lines)
  // and the error lines to flag.
  const { items, validLines, errorLines } = useMemo(() => {
    const items: ParsedBatchItem[] = []
    const validLines: string[] = []
    const errorLines: { line: number; reason: string }[] = []
    const rawLines = text.split('\n')
    rawLines.forEach((raw, i) => {
      const r = parseBatchLine(raw)
      if (r.error === 'skip') return
      if (!r.ok || !r.item) {
        errorLines.push({ line: i + 1, reason: r.error ?? 'invalidSyntax' })
        return
      }
      items.push(r.item)
      validLines.push(raw.trim())
    })
    return { items, validLines, errorLines }
  }, [text])

  const reasonText = (reason: string): string => {
    switch (reason) {
      case 'invalidUrl':
        return t('form.invalidUrl')
      case 'invalidDate':
        return t('form.invalidDate')
      default:
        return t('form.batchSyntax')
    }
  }

  const submit = async (linesToSend: string[], itemsToSend: ImportItem[]) => {
    if (busy) return
    setBusy(true)
    try {
      const res = await api.batchCreate(itemsToSend)
      const failed: Failure[] = []
      const kept: string[] = []
      linesToSend.forEach((line, i) => {
        const r = res.results[i]
        if (r?.status === 'created') {
          onCreated()
        } else {
          failed.push({
            line,
            reason: r?.error_code === 'code_taken' ? t('form.codeTaken') : r?.error_message ?? t('common.error'),
          })
          kept.push(line)
        }
      })
      setFailures(failed)
      // Successful lines leave the input; failed ones stay editable.
      setText(kept.join('\n'))
      if (res.created > 0) {
        toast(t('form.batchDone', { created: res.created, failed: res.failed }))
      }
    } catch (err) {
      toast(err instanceof ApiError ? err.message : t('common.error'), 'error')
    } finally {
      setBusy(false)
    }
  }

  const retry = () => {
    const still = items
    if (still.length === 0 || busy) return
    void submit(validLines, still)
  }

  const close = () => {
    setText('')
    setFailures([])
    onClose()
  }

  return (
    <Dialog open={open} onClose={close} title={t('form.batchTitle')} wide>
      <div className="flex flex-col gap-3">
        <div>
          <Label htmlFor="batch-lines">{t('form.batchHint')}</Label>
          <Textarea
            id="batch-lines"
            rows={8}
            value={text}
            onChange={(e) => {
              setText(e.target.value)
              setFailures([])
            }}
            className="short-code"
            placeholder="[mycode](2030/12/31)https://example.com/page"
          />
        </div>

        {errorLines.length > 0 && (
          <ul className="max-h-28 space-y-1 overflow-y-auto rounded-lg border border-danger/20 bg-danger/5 px-3 py-2 text-xs text-danger">
            {errorLines.map((e) => (
              <li key={e.line}>
                {t('form.batchLineNum', { line: e.line })}：{reasonText(e.reason)}
              </li>
            ))}
          </ul>
        )}

        {failures.length > 0 && (
          <div className="rounded-lg border border-hairline bg-black/[0.02] px-3 py-2 dark:bg-white/[0.03]">
            <p className="mb-1.5 text-xs font-medium text-muted">{t('form.batchFailures')}</p>
            <ul className="max-h-28 space-y-1 overflow-y-auto text-xs">
              {failures.map((f, i) => (
                <li key={i} className="flex items-baseline justify-between gap-3">
                  <span className="short-code min-w-0 truncate">{f.line}</span>
                  <span className="shrink-0 text-muted">{f.reason}</span>
                </li>
              ))}
            </ul>
          </div>
        )}

        <div className="mt-1 flex justify-end gap-2">
          <Button variant="ghost" onClick={close}>
            {t('form.cancel')}
          </Button>
          {failures.length > 0 && items.length > 0 && (
            <Button variant="outline" onClick={retry} disabled={busy}>
              {t('form.batchRetry')}
            </Button>
          )}
          <Button
            onClick={() => void submit(validLines, items)}
            disabled={busy || items.length === 0 || errorLines.length > 0}
          >
            {t('form.create')}
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
