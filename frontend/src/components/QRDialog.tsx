import { useEffect, useState } from 'react'
import { QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'
import { Dialog } from './ui'
import type { Link } from '../lib/api'

// Shows the QR code for a link. With multiple base URLs every variant is
// switchable — each URL carries its own code.
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
  // The parent nulls the link on close; keep the last content so the QR does
  // not vanish while the dialog is still animating out.
  const [shown, setShown] = useState(link)
  useEffect(() => {
    if (link) setShown(link)
  }, [link])
  const urls = shown?.urls ?? []
  const [index, setIndex] = useState(0)
  useEffect(() => {
    if (open && link) setIndex(Math.min(initialIndex, Math.max(urls.length - 1, 0)))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, link, initialIndex])
  const active = urls[Math.min(index, Math.max(urls.length - 1, 0))]

  return (
    <Dialog open={open} onClose={onClose} title={t('links.qr')}>
      {shown && (
        <div className="flex flex-col items-center gap-4">
          <div className="short-code text-sm text-muted">{shown.code}</div>
          {active ? (
            <div className="rounded-2xl bg-white p-4 shadow-inner">
              <QRCodeSVG value={active} size={196} fgColor="#1d1d1f" />
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
