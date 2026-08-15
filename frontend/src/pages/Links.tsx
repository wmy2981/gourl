import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  Check,
  ChevronLeft,
  ChevronRight,
  Copy,
  Download,
  FilePlus2,
  Pencil,
  Plus,
  QrCode,
  Search,
  Trash2,
} from 'lucide-react'
import { api, ApiError, type Link } from '../lib/api'
import { Button, Card, Dialog, Input, useToast } from '../components/ui'
import LinkFormDialog from '../components/LinkFormDialog'
import QRDialog from '../components/QRDialog'
import ImportDialog from '../components/ImportDialog'

const PAGE_SIZE = 20

function copyText(text: string): Promise<void> {
  return navigator.clipboard.writeText(text)
}

function formatDate(unix: number, t: (k: string) => string): string {
  if (unix <= 0) return t('common.never')
  return new Date(unix * 1000).toLocaleString()
}

export default function Links() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const queryClient = useQueryClient()

  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Link | null>(null)
  const [qrLink, setQrLink] = useState<Link | null>(null)
  const [importOpen, setImportOpen] = useState(false)
  const [deleting, setDeleting] = useState<Link | null>(null)
  const [copied, setCopied] = useState('')

  const { data, isLoading, isError } = useQuery({
    queryKey: ['links', query, page],
    queryFn: () => api.listLinks({ q: query, page, page_size: PAGE_SIZE }),
    placeholderData: (prev) => prev,
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['links'] })

  const deleteMutation = useMutation({
    mutationFn: (code: string) => api.deleteLink(code),
    onSuccess: () => {
      invalidate()
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
      setDeleting(null)
      toast(t('links.delete'))
    },
    onError: (err: unknown) => toast(err instanceof ApiError ? err.message : t('common.error'), 'error'),
  })

  const copy = async (code: string, url: string) => {
    try {
      await copyText(url)
      setCopied(code)
      setTimeout(() => setCopied(''), 1500)
    } catch {
      toast(t('common.error'), 'error')
    }
  }

  const exportCsv = async () => {
    try {
      const res = await fetch('/api/v1/export.csv', { credentials: 'same-origin' })
      if (!res.ok) throw new Error(String(res.status))
      const blob = await res.blob()
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = `gourl-links-${new Date().toISOString().slice(0, 10)}.csv`
      a.click()
      URL.revokeObjectURL(a.href)
    } catch {
      toast(t('common.error'), 'error')
    }
  }

  const totalPages = data ? Math.max(1, Math.ceil(data.total / PAGE_SIZE)) : 1

  return (
    <div>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold tracking-tight">{t('links.heading')}</h1>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={() => setImportOpen(true)}>
            <FilePlus2 size={16} />
            {t('links.import')}
          </Button>
          <Button variant="outline" onClick={exportCsv}>
            <Download size={16} />
            {t('links.export')}
          </Button>
          <Button
            onClick={() => {
              setEditing(null)
              setFormOpen(true)
            }}
          >
            <Plus size={16} />
            {t('links.create')}
          </Button>
        </div>
      </div>

      <div className="relative mb-4 max-w-md">
        <Search size={16} className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-muted" />
        <Input
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            setPage(1)
          }}
          placeholder={t('links.search')}
          className="pl-9"
        />
      </div>

      {isLoading ? (
        <p className="py-16 text-center text-muted">{t('common.loading')}</p>
      ) : isError || !data ? (
        <p className="py-16 text-center text-muted">{t('common.error')}</p>
      ) : data.links.length === 0 ? (
        <Card className="py-16 text-center text-muted">
          {query ? t('links.noResults') : t('links.empty')}
        </Card>
      ) : (
        <Card className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-left text-sm">
            <thead>
              <tr className="border-b border-hairline text-xs uppercase tracking-wide text-muted">
                <th className="px-5 py-3 font-medium">{t('links.shortUrl')}</th>
                <th className="px-5 py-3 font-medium">{t('links.destination')}</th>
                <th className="px-5 py-3 text-right font-medium">{t('links.clicks')}</th>
                <th className="px-5 py-3 font-medium">{t('links.expires')}</th>
                <th className="px-5 py-3 font-medium">{t('links.actions')}</th>
              </tr>
            </thead>
            <tbody>
              {data.links.map((link) => (
                <tr key={link.code} className="border-b border-hairline/60 last:border-0 hover:bg-black/[0.02] dark:hover:bg-white/[0.03]">
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-1.5">
                      <button
                        onClick={() => setQrLink(link)}
                        title={t('links.qr')}
                        className="rounded-md p-1 text-muted transition-colors hover:bg-accent-soft hover:text-accent-deep dark:hover:text-accent"
                      >
                        <QrCode size={15} />
                      </button>
                      <span className="short-code font-medium">{link.code}</span>
                      {link.urls[0] && (
                        <button
                          onClick={() => copy(link.code, link.urls[0])}
                          title={t('links.copy')}
                          className="rounded-md p-1 text-muted transition-colors hover:bg-accent-soft hover:text-accent-deep dark:hover:text-accent"
                        >
                          {copied === link.code ? <Check size={15} className="text-success" /> : <Copy size={15} />}
                        </button>
                      )}
                    </div>
                    {link.title && <div className="mt-0.5 max-w-[220px] truncate text-xs text-muted">{link.title}</div>}
                  </td>
                  <td className="max-w-[280px] px-5 py-3">
                    <div className="route-line mb-1.5" />
                    <a href={link.url} target="_blank" rel="noreferrer" className="block truncate text-muted transition-colors hover:text-accent-deep dark:hover:text-accent">
                      {link.url}
                    </a>
                  </td>
                  <td className="short-code px-5 py-3 text-right tabular-nums">{link.click_count}</td>
                  <td className="px-5 py-3 text-muted">{formatDate(link.expires_at, t)}</td>
                  <td className="px-5 py-3">
                    <div className="flex gap-1">
                      <button
                        onClick={() => {
                          setEditing(link)
                          setFormOpen(true)
                        }}
                        title={t('links.edit')}
                        className="rounded-md p-1.5 text-muted transition-colors hover:bg-black/5 dark:hover:bg-white/10"
                      >
                        <Pencil size={15} />
                      </button>
                      <button
                        onClick={() => setDeleting(link)}
                        title={t('links.delete')}
                        className="rounded-md p-1.5 text-muted transition-colors hover:bg-danger/10 hover:text-danger"
                      >
                        <Trash2 size={15} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          {totalPages > 1 && (
            <div className="flex items-center justify-between border-t border-hairline px-5 py-3 text-sm text-muted">
              <span>
                {t('links.page')} {page} {t('links.of')} {totalPages} · {data.total} {t('links.total')}
              </span>
              <div className="flex gap-1">
                <Button variant="ghost" className="!p-1.5" disabled={page <= 1} onClick={() => setPage(page - 1)} aria-label="Previous">
                  <ChevronLeft size={16} />
                </Button>
                <Button variant="ghost" className="!p-1.5" disabled={page >= totalPages} onClick={() => setPage(page + 1)} aria-label="Next">
                  <ChevronRight size={16} />
                </Button>
              </div>
            </div>
          )}
        </Card>
      )}

      <LinkFormDialog
        link={editing}
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSaved={() => {
          invalidate()
          queryClient.invalidateQueries({ queryKey: ['dashboard'] })
        }}
      />
      <QRDialog link={qrLink} open={qrLink !== null} onClose={() => setQrLink(null)} />
      <ImportDialog open={importOpen} onClose={() => setImportOpen(false)} onImported={invalidate} />

      <Dialog open={deleting !== null} onClose={() => setDeleting(null)} title={t('links.delete')}>
        <p className="text-sm text-muted">{t('links.deleteConfirm')}</p>
        {deleting && (
          <p className="short-code mt-2 text-sm font-medium">{deleting.code}</p>
        )}
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" onClick={() => setDeleting(null)}>
            {t('common.cancel')}
          </Button>
          <Button
            variant="danger"
            disabled={deleteMutation.isPending}
            onClick={() => deleting && deleteMutation.mutate(deleting.code)}
          >
            {t('common.delete')}
          </Button>
        </div>
      </Dialog>
    </div>
  )
}
