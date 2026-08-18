import { useEffect, useRef, useState } from 'react'
import { QRCodeCanvas, QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'
import { Download } from 'lucide-react'
import { Dialog, useToast } from './ui'
import { blobToBase64, saveDownload } from '../lib/download'
import type { Link } from '../lib/api'
import iconUrl from '../assets/icon.svg'

// Download layout, fixed light so the JPEG stays readable anywhere: a white
// rounded card on the light console tone, an amber brand badge + service name
// on top, the black-on-white QR matrix, the short code beneath — the same
// look the dialog shows on screen.
const DL_W = 340
const DL_QR = 300
// Balanced card layout: the brand zone is tall enough that the centered
// badge sits well inside the panel top (16px, the old 64px left only 3px),
// and the code sits just under the QR with a small gap (baseline 9px below
// it), leaving the rest of the footer zone as air below the code.
const DL_HEAD = 90
const DL_FOOT = 30
const DL_NAME = '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'

// Shows the QR code for a link. With multiple base URLs every variant is
// switchable — each URL carries its own code. The header download button
// saves the current variant as a JPEG, named {code}.jpg.
export default function QRDialog({
  link,
  urls,
  open,
  onClose,
  initialIndex = 0,
  siteName,
}: {
  link: Link | null
  /** Complete short URLs for the link, assembled by the parent from config. */
  urls: string[]
  open: boolean
  onClose: () => void
  /** Base URL picked on the links row; the dialog opens on that variant. */
  initialIndex?: number
  /** Service name printed on the downloaded card (from the site config). */
  siteName?: string
}) {
  const { t } = useTranslation()
  const { toast } = useToast()
  // Hidden canvas twin of the visible SVG, used only for the JPEG export
  // (an SVG cannot be rasterized without a library).
  const canvasRef = useRef<HTMLCanvasElement>(null)

  const drawBrand = (ctx: CanvasRenderingContext2D, img: HTMLImageElement, w: number) => {
    const badge = 30
    const gap = 10
    ctx.font = `600 15px ${DL_NAME}`
    const name = (siteName || 'gourl').slice(0, 24)
    const nameW = ctx.measureText(name).width
    const x0 = (w - badge - gap - nameW) / 2
    const y0 = (DL_HEAD - badge) / 2
    ctx.beginPath()
    ctx.roundRect(x0, y0, badge, badge, 10)
    ctx.fillStyle = '#fef3c7' // soft amber badge
    ctx.fill()
    ctx.drawImage(img, x0 + 5, y0 + 5, badge - 10, badge - 10)
    ctx.fillStyle = '#1d1d1f'
    ctx.textAlign = 'left'
    ctx.textBaseline = 'middle'
    ctx.fillText(name, x0 + badge + gap, y0 + badge / 2)
  }

  const drawBody = (ctx: CanvasRenderingContext2D, src: HTMLCanvasElement) => {
    ctx.drawImage(src, (DL_W - DL_QR) / 2, DL_HEAD)
    ctx.fillStyle = '#1d1d1f'
    ctx.font = '12px ui-monospace, SFMono-Regular, Menlo, monospace'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'alphabetic'
    ctx.fillText(shown?.code ?? '', DL_W / 2, DL_HEAD + DL_QR + 9)
  }

  const download = async () => {
    const src = canvasRef.current
    if (!src) return
    const out = document.createElement('canvas')
    out.width = DL_W
    out.height = DL_HEAD + DL_QR + DL_FOOT
    const ctx = out.getContext('2d')
    if (!ctx) return
    // Card: white rounded panel with a hairline border on the light canvas tone.
    const pad = 14
    ctx.fillStyle = '#f5f5f7'
    ctx.fillRect(0, 0, out.width, out.height)
    ctx.beginPath()
    ctx.roundRect(pad, pad, out.width - 2 * pad, out.height - 2 * pad, 20)
    ctx.fillStyle = '#ffffff'
    ctx.fill()
    ctx.strokeStyle = '#e5e5ea'
    ctx.lineWidth = 1
    ctx.stroke()
    // The brand icon is an SVG asset; load it through a blob: URL so the
    // canvas is never tainted (a tainted canvas makes toBlob/toDataURL fail
    // or return empty in the WebView — the ERR_PARAM_DATA_INVALID downloads).
    // Rasterize it, then paint the QR (the code survives icon failures).
    let img: HTMLImageElement | null = null
    try {
      const res = await fetch(iconUrl)
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      img = await new Promise<HTMLImageElement>((resolve, reject) => {
        const i = new Image()
        i.onload = () => resolve(i)
        i.onerror = () => reject(new Error('brand icon failed to load'))
        i.src = url
      })
    } catch {
      // Fall through to the icon-less card.
    }
    if (img) drawBrand(ctx, img, out.width)
    drawBody(ctx, src)
    // toBlob → base64 instead of toDataURL string slicing: the WebView can
    // answer toDataURL with an empty "data:," that decodes to nothing.
    const blob = await new Promise<Blob | null>((resolve) => out.toBlob(resolve, 'image/jpeg', 0.92))
    if (!blob) {
      toast(t('common.error'), 'error')
      return
    }
    const base64 = await blobToBase64(blob)
    if (!base64) {
      toast(t('common.error'), 'error')
      return
    }
    // In the app the JPEG is written to Downloads/gourl/ (path toasted);
    // on the web the browser downloads it as before.
    try {
      const path = await saveDownload(`${shown?.code ?? 'qr'}.jpg`, base64, true, 'image/jpeg')
      if (path) toast(t('links.downloadedTo', { path }))
    } catch (err) {
      // Surface the plugin error (e.g. ERR_FILE_SAVE_FAILED) so native
      // download failures are diagnosable from the UI.
      toast(err instanceof Error ? err.message : t('common.error'), 'error')
    }
  }
  // The parent nulls the link on close; keep the last content so the QR does
  // not vanish while the dialog is still animating out.
  const [shown, setShown] = useState(link)
  useEffect(() => {
    if (link) setShown(link)
  }, [link])
  const [index, setIndex] = useState(0)
  useEffect(() => {
    // Clamp against the *incoming* link's URLs: on the first render `shown`
    // still holds null (or the previous link), so clamping against `urls`
    // would drop the picked variant back to 0 — and on reopen it would clamp
    // with the previous link's URL count.
    if (open && link) setIndex(Math.min(initialIndex, Math.max(urls.length - 1, 0)))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, link, initialIndex, urls])
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
          // JPEG download hidden for now — the handler stays for a later re-enable.
          hidden
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
                size={300}
                marginSize={8}
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
