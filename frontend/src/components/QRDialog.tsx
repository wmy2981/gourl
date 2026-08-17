import { useEffect, useRef, useState } from 'react'
import { QRCodeCanvas, QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'
import { Download } from 'lucide-react'
import { Dialog } from './ui'
import type { Link } from '../lib/api'

// Shows the QR code for a link. With multiple base URLs every variant is
// switchable — each URL carries its own code. The header download button
// saves the current variant as a JPEG: white background, the short code
// printed beneath the matrix, named {code}.jpg.
export default function QRDialog({
  link,
  open,
  onClose,
  initialIndex = 0,
}: {
  link: Link | null
  open: boolean
  onClose: () => void
  /** Base URL picked on the links row; the dialog opens on that variant. */
  initialIndex?: number
}) {
  const { t } = useTranslation()
  // Hidden canvas twin of the visible SVG, used only for the JPEG export
  // (an SVG cannot be rasterized without a library).
  const canvasRef = useRef<HTMLCanvasElement>(null)

  const download = () => {
    const src = canvasRef.current
    if (!src) return
    const textH = 28
    const out = document.createElement('canvas')
    out.width = src.width
    out.height = src.height + textH
    const ctx = out.getContext('2d')
    if (!ctx) return
    ctx.fillStyle = '#ffffff'
    ctx.fillRect(0, 0, out.width, out.height)
    ctx.drawImage(src, 0, 0)
    ctx.fillStyle = '#1d1d1f'
    ctx.font = '12px ui-monospace, SFMono-Regular, Menlo, monospace'
    ctx.textAlign = 'center'
    ctx.fillText(shown?.code ?? '', out.width / 2, out.height - 10)
    const a = document.createElement('a')
    a.href = out.toDataURL('image/jpeg', 0.92)
    a.download = `${shown?.code ?? 'qr'}.jpg`
    a.click()
  }
  // The parent nulls the link on close; keep the last content so the QR does
  // not vanish while the dialog is still animating out.
  const [shown, setShown] = useState(link)
  useEffect(() => {
    if (link) setShown(link)
  }, [link])
  const urls = shown?.urls ?? []
  const [index, setIndex] = useState(0)
  useEffect(() => {
    // Clamp against the *incoming* link's URLs: on the first render `shown`
    // still holds null (or the previous link), so clamping against `urls`
    // would drop the picked variant back to 0 — and on reopen it would clamp
    // with the previous link's URL count.
    if (open && link) setIndex(Math.min(initialIndex, Math.max(link.urls.length - 1, 0)))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, link, initialIndex])
  const active = urls[Math.min(index, Math.max(urls.length - 1, 0))]

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={t('links.qr')}
      headerActions={
        <button
          onClick={download}
          disabled={!active}
          aria-label={t('links.qrDownload')}
          className="rounded-lg p-1 transition-colors hover:bg-black/5 disabled:opacity-40 dark:hover:bg-white/10"
        >
          <Download size={16} />
        </button>
      }
    >
      {shown && (
        <div className="flex flex-col items-center gap-4">
          <div className="short-code text-sm text-muted">{shown.code}</div>
          {active ? (
            <div className="rounded-2xl bg-white p-4 shadow-inner">
              <QRCodeSVG value={active} size={196} fgColor="#1d1d1f" />
              <QRCodeCanvas
                ref={canvasRef}
                value={active}
                size={196}
                fgColor="#1d1d1f"
                bgColor="#ffffff"
                style={{ display: 'none' }}
              />
            </div>
          ) : (
            <p className="py-8 text-sm text-muted">{t('common.error')}</p>
          )}
          {urls.length > 1 && (
            <div className="flex max-w-full flex-wrap justify-center gap-1.5">
              {urls.map((u, i) => (
                <button
                  key={u}
                  onClick={() => setIndex(i)}
                  className={`rounded-lg px-2 py-1 text-xs transition-colors ${
                    i === index
                      ? 'bg-accent-soft font-medium text-accent-deep dark:text-accent'
                      : 'text-muted hover:bg-black/5 dark:hover:bg-white/10'
                  }`}
                >
                  {new URL(u).host}
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </Dialog>
  )
}
