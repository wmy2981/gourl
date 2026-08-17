import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  Check,
  ChevronDown,
  ChevronLeft,
  CalendarX,
  ChevronRight,
  Copy,
  Download,
  FilePlus2,
  ListPlus,
  Pencil,
  Plus,
  QrCode,
  Search,
  Trash2,
} from 'lucide-react'
import { api, ApiError, linkUrls, type Link } from '../lib/api'
import { copyText } from '../lib/clipboard'
import { Button, Card, Checkbox, Dialog, Input, Select, useToast } from '../components/ui'
import LinkFormDialog from '../components/LinkFormDialog'
import QRDialog from '../components/QRDialog'
import ImportDialog from '../components/ImportDialog'
import ExportDialog from '../components/ExportDialog'
import BatchCreateDialog from '../components/BatchCreateDialog'

const PAGE_SIZE = 20

function formatDate(unix: number, t: (k: string) => string): string {
  if (unix <= 0) return t('common.never')
  const d = new Date(unix * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}/${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function isExpired(link: Link): boolean {
  return link.expires_at > 0 && link.expires_at * 1000 < Date.now()
}

export default function Links() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const queryClient = useQueryClient()

  const [query, setQuery] = useState('')
  const [expires, setExpires] = useState('')
  const [page, setPage] = useState(1)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Link | null>(null)
  const [qrLink, setQrLink] = useState<Link | null>(null)
  const [importOpen, setImportOpen] = useState(false)
  const [exportOpen, setExportOpen] = useState(false)
  const [batchOpen, setBatchOpen] = useState(false)
  const [deleting, setDeleting] = useState<Link | null>(null)
  // Batch selection: codes chosen across pages are kept until the search
  // changes (or the deletion finishes), so mis-scoped selections can't linger.
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false)
  const [expiredConfirm, setExpiredConfirm] = useState<number | null>(null)
  const [copied, setCopied] = useState('')
  // Per-link pick of which base URL is shown/copied; defaults to the first.
  const [urlIdx, setUrlIdx] = useState<Record<string, number>>({})
  const [urlMenu, setUrlMenu] = useState<string | null>(null)
  // The menu is rendered through a portal into document.body with fixed
  // viewport coordinates: nothing inside the page (overflow containers,
  // transformed wrappers, stacking contexts) can clip or hijack it. These
  // hold the viewport origin it was opened at.
  const [menuPos, setMenuPos] = useState<{ x: number; y: number } | null>(null)
  const [menuClosing, setMenuClosing] = useState(false)
  const openedAt = useRef(0)

  const closeUrlMenu = () => {
    if (menuClosing) return
    setMenuClosing(true)
    setTimeout(() => {
      setMenuClosing(false)
      setUrlMenu(null)
      setMenuPos(null)
    }, 160)
  }

  // A fixed menu drifts away from its button on scroll — close it instead.
  // The 150ms grace window ignores the scroll burst a click itself can cause
  // (focus scrolling, inertia) right after opening.
  useEffect(() => {
    if (!urlMenu) return
    const onScroll = () => {
      if (Date.now() - openedAt.current < 150) return
      setUrlMenu(null)
      setMenuPos(null)
    }
    window.addEventListener('scroll', onScroll, true)
    return () => window.removeEventListener('scroll', onScroll, true)
  }, [urlMenu])

  const { data, isLoading, isError } = useQuery({
    queryKey: ['links', query, expires, page],
    queryFn: () => api.listLinks({ q: query, expires, page, page_size: PAGE_SIZE }),
    placeholderData: (prev) => prev,
  })

  // Short URLs are assembled here from the config (base URL + extras); the
  // backend no longer returns them per link.
  const { data: cfg } = useQuery({ queryKey: ['config'], queryFn: api.getConfig })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['links'] })

  const deleteMutation = useMutation({
    mutationFn: (code: string) => api.deleteLink(code),
    onSuccess: () => {
      invalidate()
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
      setDeleting(null)
      toast(t('links.deleted'))
    },
    onError: (err: unknown) => toast(err instanceof ApiError ? err.message : t('common.error'), 'error'),
  })

  const pageCodes = data?.links.map((l) => l.code) ?? []
  const allPageSelected = pageCodes.length > 0 && pageCodes.every((c) => selected.has(c))

  const togglePage = (on: boolean) => {
    const next = new Set(selected)
    for (const c of pageCodes) {
      if (on) next.add(c)
      else next.delete(c)
    }
    setSelected(next)
  }

  const deleteSelectedMutation = useMutation({
    mutationFn: (codes: string[]) => api.deleteLinks(codes),
    onSuccess: () => {
      invalidate()
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
      setBulkDeleteOpen(false)
      setSelected(new Set())
      toast(t('links.deleted'))
    },
    onError: (err: unknown) => toast(err instanceof ApiError ? err.message : t('common.error'), 'error'),
  })

  const clearExpired = async () => {
    try {
      const { count } = await api.expiredCount()
      if (count === 0) {
        toast(t('links.noExpired'))
        return
      }
      setExpiredConfirm(count)
    } catch {
      toast(t('common.error'), 'error')
    }
  }

  const clearExpiredMutation = useMutation({
    mutationFn: () => api.deleteExpired(),
    onSuccess: (res) => {
      invalidate()
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
      setExpiredConfirm(null)
      toast(t('links.expiredCleared', { count: res.deleted }))
    },
    onError: (err: unknown) => toast(err instanceof ApiError ? err.message : t('common.error'), 'error'),
  })

  const copy = async (code: string, url: string) => {
    // Clipboard fallback chain: API → hidden textarea (works on plain http).
    const ok = await copyText(url)
    if (ok) {
      setCopied(code)
      setTimeout(() => setCopied(''), 1500)
    } else {
      toast(url, 'error')
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
          <Button variant="outline" onClick={() => setExportOpen(true)}>
            <Download size={16} />
            {t('links.export')}
          </Button>
          <Button variant="outline" onClick={() => setBatchOpen(true)}>
            <ListPlus size={16} />
            {t('links.batchCreate')}
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

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <div className="relative max-w-md flex-1">
          <Search size={16} className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-muted" />
          <Input
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setPage(1)
              // A different search context invalidates the cross-page selection.
              setSelected(new Set())
            }}
            placeholder={t('links.search')}
            className="pl-9"
          />
        </div>
        <Select
          value={expires}
          onChange={(v) => {
            setExpires(v)
            setPage(1)
            setSelected(new Set())
          }}
          ariaLabel={t('links.filterExpires')}
          options={[
            { value: '', label: t('links.filterAll') },
            { value: 'active', label: t('links.filterActive') },
            { value: 'expired', label: t('links.filterExpired') },
          ]}
          className="w-32"
        />
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" onClick={clearExpired}>
            <CalendarX size={16} />
            {t('links.clearExpired')}
          </Button>
          {selected.size > 0 && (
            <>
              <span className="text-sm tabular-nums text-muted">{t('links.selected', { count: selected.size })}</span>
              <Button variant="danger" onClick={() => setBulkDeleteOpen(true)}>
                <Trash2 size={16} />
                {t('links.deleteSelected')}
              </Button>
            </>
          )}
        </div>
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
          <table className="w-full min-w-[900px] text-left text-sm">
            <thead>
              <tr className="border-b border-hairline text-xs uppercase tracking-wide text-muted">
                <th className="w-12 whitespace-nowrap px-5 py-3">
                  <Checkbox checked={allPageSelected} onChange={togglePage} aria-label={t('links.selectAll')} />
                </th>
                <th className="whitespace-nowrap px-5 py-3 font-medium">{t('links.shortUrl')}</th>
                <th className="whitespace-nowrap px-5 py-3 font-medium">{t('links.destination')}</th>
                <th className="whitespace-nowrap px-5 py-3 font-medium">{t('links.description')}</th>
                <th className="whitespace-nowrap px-5 py-3 text-right font-medium">{t('links.clicks')}</th>
                <th className="whitespace-nowrap px-5 py-3 font-medium">{t('links.expires')}</th>
                <th className="whitespace-nowrap px-5 py-3 font-medium">{t('links.actions')}</th>
              </tr>
            </thead>
            <tbody>
              {data.links.map((link) => {
                const urls = cfg ? linkUrls(link.code, cfg) : []
                const idx = urlIdx[link.code] ?? 0
                const current = urls[idx] ?? urls[0]
                return (
                <tr key={link.code} className="border-b border-hairline/60 last:border-0 hover:bg-black/[0.02] dark:hover:bg-white/[0.03]">
                  <td className="px-5 py-3">
                    <Checkbox
                      checked={selected.has(link.code)}
                      onChange={(on) => {
                        const next = new Set(selected)
                        if (on) next.add(link.code)
                        else next.delete(link.code)
                        setSelected(next)
                      }}
                      aria-label={t('links.selectRow')}
                    />
                  </td>
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
                      {current && (
                        <button
                          onClick={() => copy(link.code, current)}
                          title={t('links.copy')}
                          className="rounded-md p-1 text-muted transition-colors hover:bg-accent-soft hover:text-accent-deep dark:hover:text-accent"
                        >
                          {copied === link.code ? <Check size={15} className="text-success" /> : <Copy size={15} />}
                        </button>
                      )}
                    </div>
                    {current && (
                      <div className="relative mt-0.5">
                        <button
                          onClick={(e) => {
                            if (urlMenu === link.code) {
                              closeUrlMenu()
                              return
                            }
                            const r = e.currentTarget.getBoundingClientRect()
                            // Clamp right edge; flip above the button when
                            // there is no room below it in the viewport.
                            const estH = 200
                            const fitsBelow = r.bottom + 4 + estH <= window.innerHeight
                            setMenuPos({
                              x: Math.min(r.left, window.innerWidth - 336),
                              y: fitsBelow ? r.bottom + 4 : Math.max(8, r.top - estH - 4),
                            })
                            openedAt.current = Date.now()
                            setUrlMenu(link.code)
                          }}
                          aria-label={t('links.pickBaseUrl')}
                          className="flex max-w-[240px] items-center gap-1 text-xs text-muted transition-colors hover:text-accent-deep dark:hover:text-accent"
                        >
                          <span className="truncate">{current}</span>
                          {urls.length > 1 && (
                            <ChevronDown
                              size={12}
                              className={`shrink-0 opacity-60 transition-transform duration-200 ${
                                urlMenu === link.code ? 'rotate-180' : ''
                              }`}
                            />
                          )}
                        </button>
                        {urlMenu === link.code &&
                          menuPos &&
                          urls.length > 1 &&
                          createPortal(
                            <>
                              <div className="fixed inset-0 z-40" onClick={closeUrlMenu} />
                              <div
                                className={`fixed z-50 w-80 rounded-xl border border-hairline bg-white p-1 shadow-[0_12px_40px_rgba(0,0,0,0.18)] dark:bg-[#1c1c1e] ${
                                  menuClosing ? 'animate-pop-out' : 'animate-pop-in'
                                }`}
                                style={{ left: menuPos.x, top: menuPos.y }}
                              >
                                {urls.map((u, i) => (
                                  <button
                                    key={u}
                                    onClick={() => {
                                      setUrlIdx((m) => ({ ...m, [link.code]: i }))
                                      closeUrlMenu()
                                    }}
                                    className={`flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs transition-colors ${
                                      i === idx
                                        ? 'bg-accent-soft font-medium text-accent-deep dark:text-accent'
                                        : 'text-muted hover:bg-black/5 dark:hover:bg-white/10'
                                    }`}
                                  >
                                    <span className="min-w-0 flex-1 truncate">{u}</span>
                                    {i === idx && <Check size={12} className="shrink-0" />}
                                  </button>
                                ))}
                              </div>
                            </>,
                            document.body,
                          )}
                      </div>
                    )}
                  </td>
                  <td className="max-w-[280px] px-5 py-3">
                    <div className="route-line mb-1.5" />
                    <a href={link.url} target="_blank" rel="noreferrer" className="block truncate text-muted transition-colors hover:text-accent-deep dark:hover:text-accent">
                      {link.url}
                    </a>
                    {link.title && <div className="mt-0.5 max-w-[240px] truncate text-xs text-muted/80">{link.title}</div>}
                  </td>
                  <td className="max-w-[220px] px-5 py-3 text-muted/80">
                    {link.description ? (
                      <span className="line-clamp-2" title={link.description}>
                        {link.description}
                      </span>
                    ) : (
                      <span className="text-muted/40">—</span>
                    )}
                  </td>
                  <td className="short-code px-5 py-3 text-right tabular-nums">{link.click_count}</td>
                  <td className={`px-5 py-3 ${isExpired(link) ? 'text-danger' : 'text-muted'}`}>
                    {formatDate(link.expires_at, t)}
                  </td>
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
                )
              })}
            </tbody>
          </table>

          {totalPages > 1 && (
            <div className="sticky left-0 flex items-center justify-between border-t border-hairline bg-white/90 px-5 py-3 text-sm text-muted backdrop-blur-xl dark:bg-[#161617]/90">
              <span>
                {t('links.page')} {page} {t('links.of')} {totalPages} · {data.total} {t('links.total')}
              </span>
              <div className="flex gap-1">
                <Button variant="ghost" className="!p-1.5" disabled={page <= 1} onClick={() => setPage(page - 1)} aria-label={t('links.prevPage')}>
                  <ChevronLeft size={16} />
                </Button>
                <Button variant="ghost" className="!p-1.5" disabled={page >= totalPages} onClick={() => setPage(page + 1)} aria-label={t('links.nextPage')}>
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
      {/* The QR dialog opens on the base URL picked via the row dropdown. */}
      <QRDialog
        link={qrLink}
        urls={qrLink && cfg ? linkUrls(qrLink.code, cfg) : []}
        open={qrLink !== null}
        onClose={() => setQrLink(null)}
        initialIndex={qrLink ? (urlIdx[qrLink.code] ?? 0) : 0}
      />
      <ImportDialog open={importOpen} onClose={() => setImportOpen(false)} onImported={invalidate} />
      <ExportDialog open={exportOpen} onClose={() => setExportOpen(false)} />
      <BatchCreateDialog open={batchOpen} onClose={() => setBatchOpen(false)} onCreated={invalidate} />

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

      <Dialog open={bulkDeleteOpen} onClose={() => setBulkDeleteOpen(false)} title={t('links.bulkDeleteTitle')}>
        <p className="text-sm text-muted">{t('links.bulkDeleteConfirm', { count: selected.size })}</p>
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" onClick={() => setBulkDeleteOpen(false)}>
            {t('common.cancel')}
          </Button>
          <Button
            variant="danger"
            disabled={deleteSelectedMutation.isPending}
            onClick={() => deleteSelectedMutation.mutate([...selected])}
          >
            {t('common.delete')}
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={expiredConfirm !== null}
        onClose={() => setExpiredConfirm(null)}
        title={t('links.clearExpired')}
      >
        <p className="text-sm text-muted">{t('links.clearExpiredConfirm', { count: expiredConfirm ?? 0 })}</p>
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" onClick={() => setExpiredConfirm(null)}>
            {t('common.cancel')}
          </Button>
          <Button
            variant="danger"
            disabled={clearExpiredMutation.isPending}
            onClick={() => clearExpiredMutation.mutate()}
          >
            {t('common.delete')}
          </Button>
        </div>
      </Dialog>
    </div>
  )
}
