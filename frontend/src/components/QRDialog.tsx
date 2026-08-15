import { useState } from 'react'
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
}: {
  link: Link | null
  open: boolean
  onClose: () => void
}) {
  const { t } = useTranslation()
  const urls = link?.urls ?? []
  const [index, setIndex] = useState(0)
  const active = urls[Math.min(index, Math.max(urls.length - 1, 0))]

  return (
    <Dialog open={open} onClose={onClose} title={t('links.qr')}>
      {link && (
        <div className="flex flex-col items-center gap-4">
          <div className="short-code text-sm text-muted">{link.code}</div>
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
