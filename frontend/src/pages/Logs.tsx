import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ArrowDown, Download, Loader2, Pause, Play } from 'lucide-react'
import { api, type LogRecord } from '../lib/api'
import { Button, Card, Input, useToast } from '../components/ui'

const LEVELS = ['debug', 'info', 'warn', 'error'] as const

const levelClass: Record<string, string> = {
  debug: 'text-muted/60',
  info: 'text-accent-deep dark:text-accent',
  warn: 'text-amber-600 dark:text-amber-400',
  error: 'text-danger',
}

const levelDot: Record<string, string> = {
  debug: 'bg-muted/40',
  info: 'bg-accent',
  warn: 'bg-amber-500',
  error: 'bg-danger',
}

// One log line in the file-like export format.
function formatLogLine(r: LogRecord): string {
  const attrs = r.attrs
    ? Object.entries(r.attrs)
        .map(([k, v]) => `${k}=${typeof v === 'string' && v.includes(' ') ? `"${v}"` : String(v)}`)
        .join(' ')
    : ''
  const msg = r.message.includes(' ') ? `"${r.message}"` : r.message
  return `${r.time} level=${(r.level || 'info').toUpperCase()} msg=${msg}${attrs ? ` ${attrs}` : ''}`
}

export default function Logs() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const [records, setRecords] = useState<LogRecord[]>([])
  const [historyAvailable, setHistoryAvailable] = useState(true)
  const [historyLoading, setHistoryLoading] = useState(true)
  const [historyOffset, setHistoryOffset] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const [levels, setLevels] = useState<Set<string>>(new Set(LEVELS))
  const [keyword, setKeyword] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [paused, setPaused] = useState(false)
  const [streamDown, setStreamDown] = useState(false)
  const [autoFollow, setAutoFollow] = useState(true)
  const scrollRef = useRef<HTMLDivElement>(null)

  // History from the mirrored log file (newest first on the wire; displayed
  // oldest-first below the live stream continues).
  useEffect(() => {
    let alive = true
    api
      .logHistory(200, 0)
      .then((res) => {
        if (!alive) return
        setHistoryAvailable(res.available)
        setRecords([...res.records].reverse())
        setHistoryOffset(res.records.length)
        setHasMore(res.records.length === 200)
      })
      .catch(() => {
        if (alive) setHistoryAvailable(false)
      })
      .finally(() => alive && setHistoryLoading(false))
    return () => {
      alive = false
    }
  }, [])

  // Live stream; EventSource reconnects automatically.
  useEffect(() => {
    const es = api.logStream(
      (rec) => setRecords((rs) => [...rs, rec]),
      () => setStreamDown(true),
    )
    return () => es.close()
  }, [])

  useEffect(() => {
    if (autoFollow && !paused) {
      const el = scrollRef.current
      if (el) el.scrollTop = el.scrollHeight
    }
  }, [records, paused, autoFollow])

  const filtered = useMemo(() => {
    const kw = keyword.trim().toLowerCase()
    const fromT = from ? new Date(from).getTime() : null
    const toT = to ? new Date(to).getTime() : null
    return records.filter((r) => {
      if (r.level && !levels.has(r.level)) return false
      if (kw) {
        const hay = `${r.message} ${JSON.stringify(r.attrs ?? {})}`.toLowerCase()
        if (!hay.includes(kw)) return false
      }
      const ts = new Date(r.time).getTime()
      if (fromT !== null && ts < fromT) return false
      if (toT !== null && ts > toT) return false
      return true
    })
  }, [records, levels, keyword, from, to])

  const toggleLevel = (lv: string) => {
    setLevels((prev) => {
      const next = new Set(prev)
      if (next.has(lv)) next.delete(lv)
      else next.add(lv)
      return next
    })
  }

  const loadMore = async () => {
    try {
      const res = await api.logHistory(200, historyOffset)
      setRecords((rs) => [...[...res.records].reverse(), ...rs])
      setHistoryOffset((o) => o + res.records.length)
      setHasMore(res.records.length === 200)
    } catch {
      toast(t('common.error'), 'error')
    }
  }

  const exportLog = () => {
    const date = new Date().toISOString().slice(0, 10)
    const text = filtered.map(formatLogLine).join('\n') + (filtered.length ? '\n' : '')
    const a = document.createElement('a')
    a.href = URL.createObjectURL(new Blob([text], { type: 'text/plain' }))
    a.download = `gourl-logs-${date}.log`
    a.click()
    URL.revokeObjectURL(a.href)
  }

  const jumpToBottom = () => {
    const el = scrollRef.current
    if (el) el.scrollTop = el.scrollHeight
    setAutoFollow(true)
  }

  return (
    <div>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold tracking-tight">{t('nav.logs')}</h1>
        <div className="flex flex-wrap items-center gap-2">
          <span className={`flex items-center gap-1.5 text-xs ${streamDown ? 'text-danger' : 'text-muted'}`}>
            <span className={`size-1.5 rounded-full ${streamDown ? 'bg-danger' : 'bg-success'}`} />
            {streamDown ? t('logs.reconnecting') : t('logs.live')}
          </span>
          <Button variant="outline" onClick={() => setPaused((p) => !p)}>
            {paused ? <Play size={14} /> : <Pause size={14} />}
            {paused ? t('logs.resume') : t('logs.pause')}
          </Button>
          <Button variant="outline" onClick={exportLog}>
            <Download size={14} />
            {t('logs.export')}
          </Button>
        </div>
      </div>

      <Card className="mb-4 flex flex-wrap items-center gap-3 px-4 py-3">
        <div className="flex items-center gap-1.5">
          {LEVELS.map((lv) => (
            <button
              key={lv}
              onClick={() => toggleLevel(lv)}
              className={`rounded-lg px-2.5 py-1 text-xs font-medium transition-colors ${
                levels.has(lv)
                  ? 'bg-accent-soft text-accent-deep dark:text-accent'
                  : 'text-muted hover:bg-black/5 dark:hover:bg-white/10'
              }`}
            >
              {lv}
            </button>
          ))}
        </div>
        <Input
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          placeholder={t('logs.keyword')}
          className="w-44"
        />
        <label className="flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-muted">
          <input
            type="datetime-local"
            value={from}
            onChange={(e) => setFrom(e.target.value)}
            aria-label={t('logs.from')}
            className="w-40 bg-transparent text-xs text-ink outline-none dark:text-ink-dark"
          />
          {t('logs.to')}
          <input
            type="datetime-local"
            value={to}
            onChange={(e) => setTo(e.target.value)}
            aria-label={t('logs.to')}
            className="w-40 bg-transparent text-xs text-ink outline-none dark:text-ink-dark"
          />
        </label>
      </Card>

      {!historyAvailable && (
        <p className="mb-3 text-xs text-muted">{t('logs.noHistory')}</p>
      )}

      <div className="relative">
        <Card ref={scrollRef} className="h-[60vh] overflow-y-auto px-0 py-0" onScroll={() => {
          const el = scrollRef.current
          if (!el) return
          setAutoFollow(el.scrollHeight - el.scrollTop - el.clientHeight < 40)
        }}>
          {historyLoading ? (
            <p className="flex items-center justify-center gap-2 py-16 text-sm text-muted">
              <Loader2 size={16} className="animate-spin" />
              {t('common.loading')}
            </p>
          ) : (
            <div className="divide-y divide-hairline/60">
              {filtered.length === 0 && (
                <p className="py-16 text-center text-sm text-muted">{t('logs.empty')}</p>
              )}
              {filtered.map((r, i) => (
                <div key={i} className="flex items-start gap-3 px-4 py-1.5 font-mono text-xs leading-relaxed hover:bg-black/[0.02] dark:hover:bg-white/[0.03]">
                  <span className={`mt-1 size-1.5 shrink-0 rounded-full ${levelDot[r.level] ?? 'bg-muted/40'}`} />
                  <span className="shrink-0 tabular-nums text-muted/60">{fmtTime(r.time)}</span>
                  <span className={`shrink-0 font-medium ${levelClass[r.level] ?? 'text-muted'}`}>
                    {r.level ? r.level.toUpperCase() : '—'}
                  </span>
                  {/* flex-1 keeps the message column from collapsing to one
                      character per line on narrow viewports; the attrs column
                      is capped and hidden on phones so it can never squeeze
                      the message out. */}
                  <span className="min-w-0 flex-1 whitespace-pre-wrap break-all">{r.message}</span>
                  {r.attrs && Object.keys(r.attrs).length > 0 && (
                    <span className="ml-auto hidden max-w-[45%] shrink-0 whitespace-pre-wrap break-all text-muted/50 sm:block">
                      {Object.entries(r.attrs)
                        .map(([k, v]) => `${k}=${String(v)}`)
                        .join(' ')}
                    </span>
                  )}
                </div>
              ))}
            </div>
          )}
        </Card>
        {!autoFollow && (
          <button
            onClick={jumpToBottom}
            className="absolute bottom-4 right-4 flex items-center gap-1.5 rounded-xl border border-hairline bg-white px-3 py-1.5 text-xs font-medium shadow-[0_8px_24px_rgba(0,0,0,0.12)] transition-colors hover:bg-black/5 dark:bg-[#1c1c1e] dark:hover:bg-white/10"
          >
            <ArrowDown size={14} />
            {t('logs.latest')}
          </button>
        )}
      </div>

      {hasMore && (
        <div className="mt-3 flex justify-center">
          <Button variant="outline" onClick={loadMore}>
            {t('logs.loadMore')}
          </Button>
        </div>
      )}
    </div>
  )
}

function fmtTime(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
