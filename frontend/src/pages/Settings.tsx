import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { KeyRound, Plus, ShieldOff, Trash2, Upload } from 'lucide-react'
import { api, ApiError, type AppConfig } from '../lib/api'
import { Button, Card, Input, Label, Textarea, useToast } from '../components/ui'

export default function Settings() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const queryClient = useQueryClient()

  const { data: cfg } = useQuery({ queryKey: ['config'], queryFn: api.getConfig })
  const [form, setForm] = useState<AppConfig | null>(null)
  const [reservedText, setReservedText] = useState('')
  const [uaPattern, setUaPattern] = useState('')
  const [tokenNote, setTokenNote] = useState('')
  const [newToken, setNewToken] = useState('')

  useEffect(() => {
    if (cfg && !form) {
      setForm(cfg)
      setReservedText(cfg.reserved_codes.join('\n'))
    }
  }, [cfg, form])

  const saveMutation = useMutation({
    mutationFn: (c: AppConfig) => api.updateConfig(c),
    onSuccess: () => {
      toast(t('settings.saved'))
      queryClient.invalidateQueries({ queryKey: ['config'] })
    },
    onError: (err: unknown) =>
      toast(err instanceof ApiError ? err.message : t('settings.saveFailed'), 'error'),
  })

  if (!form) {
    return <p className="py-16 text-center text-muted">{t('common.loading')}</p>
  }

  const set = <K extends keyof AppConfig>(key: K, value: AppConfig[K]) =>
    setForm({ ...form, [key]: value })
  const setSite = (key: keyof AppConfig['site'], value: string) =>
    setForm({ ...form, site: { ...form.site, [key]: value } })

  const save = () => {
    saveMutation.mutate({
      ...form,
      reserved_codes: reservedText
        .split('\n')
        .map((s) => s.trim())
        .filter(Boolean),
    })
  }

  const iconInput = async (file: File | undefined) => {
    if (!file) return
    try {
      await api.uploadIcon(file)
      toast(t('settings.saved'))
      queryClient.invalidateQueries({ queryKey: ['config'] })
    } catch (err) {
      toast(err instanceof ApiError ? err.message : t('settings.saveFailed'), 'error')
    }
  }

  return (
    <div>
      <h1 className="mb-6 text-2xl font-semibold tracking-tight">{t('settings.heading')}</h1>

      <div className="flex flex-col gap-6">
        {/* Site information */}
        <Card className="p-6">
          <h2 className="mb-4 text-sm font-medium text-muted">{t('settings.site')}</h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <Label>{t('settings.siteName')}</Label>
              <Input value={form.site.name} onChange={(e) => setSite('name', e.target.value)} />
            </div>
            <div>
              <Label>{t('settings.siteTitle')}</Label>
              <Input value={form.site.title} onChange={(e) => setSite('title', e.target.value)} />
            </div>
            <div>
              <Label>{t('settings.keywords')}</Label>
              <Input value={form.site.keywords} onChange={(e) => setSite('keywords', e.target.value)} />
            </div>
            <div>
              <Label>{t('settings.description')}</Label>
              <Input value={form.site.description} onChange={(e) => setSite('description', e.target.value)} />
            </div>
            <div>
              <Label>{t('settings.header')}</Label>
              <Textarea rows={2} value={form.site.header} onChange={(e) => setSite('header', e.target.value)} />
            </div>
            <div>
              <Label>{t('settings.footer')}</Label>
              <Textarea rows={2} value={form.site.footer} onChange={(e) => setSite('footer', e.target.value)} />
            </div>
          </div>
        </Card>

        {/* Behavior */}
        <Card className="p-6">
          <h2 className="mb-4 text-sm font-medium text-muted">{t('settings.behavior')}</h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <Label>{t('settings.shortCodeLength')}</Label>
              <Input
                type="number"
                min={4}
                max={32}
                value={form.short_code_length}
                onChange={(e) => set('short_code_length', Number(e.target.value))}
              />
            </div>
            <div>
              <Label>{t('settings.baseUrl')}</Label>
              <Input value={form.base_url} onChange={(e) => set('base_url', e.target.value)} placeholder="https://s.example.com" />
              <p className="mt-1 text-xs text-muted">{t('settings.baseUrlHint')}</p>
            </div>
            <div className="sm:col-span-2">
              <Label>{t('settings.extraBaseUrls')}</Label>
              <div className="flex flex-col gap-2">
                {form.extra_base_urls.map((u, i) => (
                  <div key={i} className="flex gap-2">
                    <Input value={u} onChange={(e) => {
                      const next = [...form.extra_base_urls]
                      next[i] = e.target.value
                      set('extra_base_urls', next)
                    }} />
                    <Button variant="ghost" onClick={() => set('extra_base_urls', form.extra_base_urls.filter((_, j) => j !== i))} aria-label="Remove">
                      <Trash2 size={15} />
                    </Button>
                  </div>
                ))}
                <Button
                  variant="outline"
                  className="self-start"
                  onClick={() => set('extra_base_urls', [...form.extra_base_urls, ''])}
                >
                  <Plus size={15} />
                  {t('settings.addExtra')}
                </Button>
              </div>
              <p className="mt-1 text-xs text-muted">{t('settings.extraBaseUrlsHint')}</p>
            </div>
            <div className="sm:col-span-2">
              <Label>{t('settings.reservedCodes')}</Label>
              <Textarea rows={3} value={reservedText} onChange={(e) => setReservedText(e.target.value)} />
              <p className="mt-1 text-xs text-muted">{t('settings.reservedCodesHint')}</p>
            </div>
          </div>
        </Card>

        {/* Icon */}
        <Card className="p-6">
          <h2 className="mb-4 text-sm font-medium text-muted">{t('settings.icon')}</h2>
          <div className="flex flex-wrap items-center gap-3">
            <Button variant="outline" onClick={() => document.getElementById('icon-file')?.click()}>
              <Upload size={15} />
              {t('settings.uploadIcon')}
            </Button>
            <input
              id="icon-file"
              type="file"
              accept=".svg,.png"
              className="hidden"
              onChange={(e) => iconInput(e.target.files?.[0])}
            />
            {form.icon && (
              <Button variant="danger" onClick={async () => {
                await api.deleteIcon()
                toast(t('settings.saved'))
                queryClient.invalidateQueries({ queryKey: ['config'] })
              }}>
                {t('settings.removeIcon')}
              </Button>
            )}
            <span className="short-code text-xs text-muted">{form.icon || 'default'}</span>
          </div>
        </Card>

        {/* UA blocks */}
        <UABlockSection
          uaPattern={uaPattern}
          setUaPattern={setUaPattern}
          add={async () => {
            try {
              await api.addUABlock(uaPattern)
              setUaPattern('')
              queryClient.invalidateQueries({ queryKey: ['ua-blocks'] })
            } catch (err) {
              toast(err instanceof ApiError ? err.message : t('common.error'), 'error')
            }
          }}
        />

        {/* API tokens */}
        <TokenSection
          tokenNote={tokenNote}
          setTokenNote={setTokenNote}
          newToken={newToken}
          create={async () => {
            try {
              const res = await api.createToken(tokenNote)
              setNewToken(res.token)
              setTokenNote('')
              queryClient.invalidateQueries({ queryKey: ['tokens'] })
            } catch (err) {
              toast(err instanceof ApiError ? err.message : t('common.error'), 'error')
            }
          }}
        />

        <div className="flex justify-end">
          <Button onClick={save} disabled={saveMutation.isPending} className="px-8">
            {t('settings.save')}
          </Button>
        </div>
      </div>
    </div>
  )
}

function UABlockSection({
  uaPattern,
  setUaPattern,
  add,
}: {
  uaPattern: string
  setUaPattern: (v: string) => void
  add: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { data } = useQuery({ queryKey: ['ua-blocks'], queryFn: api.uaBlocks })

  return (
    <Card className="p-6">
      <h2 className="mb-1 text-sm font-medium text-muted">{t('settings.uaBlocks')}</h2>
      <p className="mb-4 text-xs text-muted">{t('settings.uaBlockHint')}</p>
      <div className="flex flex-col gap-2">
        {data?.ua_blocks.map((b) => (
          <div key={b.id} className="flex items-center justify-between rounded-xl border border-hairline px-3.5 py-2">
            <span className="short-code text-sm">{b.pattern}</span>
            <Button variant="ghost" className="!p-1.5" onClick={async () => {
              await api.deleteUABlock(b.id)
              queryClient.invalidateQueries({ queryKey: ['ua-blocks'] })
            }} aria-label="Remove">
              <ShieldOff size={15} />
            </Button>
          </div>
        ))}
        {data?.ua_blocks.length === 0 && (
          <p className="py-2 text-sm text-muted">{t('common.never')}</p>
        )}
        <div className="flex gap-2">
          <Input value={uaPattern} onChange={(e) => setUaPattern(e.target.value)} placeholder="curl" />
          <Button variant="outline" onClick={add} disabled={!uaPattern.trim()}>
            {t('settings.addUaBlock')}
          </Button>
        </div>
      </div>
    </Card>
  )
}

function TokenSection({
  tokenNote,
  setTokenNote,
  newToken,
  create,
}: {
  tokenNote: string
  setTokenNote: (v: string) => void
  newToken: string
  create: () => void
}) {
  const { t } = useTranslation()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const { data } = useQuery({ queryKey: ['tokens'], queryFn: api.tokens })

  return (
    <Card className="p-6">
      <h2 className="mb-1 text-sm font-medium text-muted">{t('settings.tokens')}</h2>
      <p className="mb-4 text-xs text-muted">{t('settings.tokensHint')}</p>
      {newToken && (
        <div className="mb-4 rounded-xl border border-accent/40 bg-accent-soft p-3">
          <p className="short-code break-all text-sm">{newToken}</p>
          <Button variant="ghost" className="mt-1 !p-1 text-xs" onClick={() => {
            navigator.clipboard.writeText(newToken)
            toast(t('links.copied'))
          }}>
            {t('links.copy')}
          </Button>
        </div>
      )}
      <div className="flex flex-col gap-2">
        {data?.tokens.map((tok) => (
          <div key={tok.id} className="flex items-center justify-between rounded-xl border border-hairline px-3.5 py-2">
            <div className="min-w-0">
              <span className="short-code text-sm">{tok.token}</span>
              {tok.note && <span className="ml-2 text-xs text-muted">{tok.note}</span>}
            </div>
            <Button variant="ghost" className="!p-1.5" onClick={async () => {
              await api.deleteToken(tok.id)
              queryClient.invalidateQueries({ queryKey: ['tokens'] })
            }} aria-label="Revoke">
              <Trash2 size={15} />
            </Button>
          </div>
        ))}
        {data?.tokens.length === 0 && (
          <p className="py-2 text-sm text-muted">
            <KeyRound size={14} className="mr-1 inline" />
            {t('common.never')}
          </p>
        )}
        <div className="flex gap-2">
          <Input value={tokenNote} onChange={(e) => setTokenNote(e.target.value)} placeholder={t('form.note')} />
          <Button variant="outline" onClick={create}>
            {t('settings.createToken')}
          </Button>
        </div>
      </div>
    </Card>
  )
}
