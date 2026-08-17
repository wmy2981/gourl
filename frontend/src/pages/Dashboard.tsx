import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Link2, MousePointerClick, Zap } from 'lucide-react'
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { api } from '../lib/api'
import { Card } from '../components/ui'

function StatCard({
  icon,
  label,
  value,
}: {
  icon: React.ReactNode
  label: string
  value: number
}) {
  return (
    <Card className="flex items-center gap-4 p-5">
      <div className="rounded-xl bg-accent-soft p-2.5 text-accent-deep dark:text-accent">
        {icon}
      </div>
      <div>
        <div className="text-xs text-muted">{label}</div>
        <div className="short-code mt-0.5 text-2xl font-semibold tabular-nums">{value.toLocaleString()}</div>
      </div>
    </Card>
  )
}

export default function Dashboard() {
  const { t } = useTranslation()
  const { data, isLoading, isError } = useQuery({
    queryKey: ['dashboard'],
    queryFn: api.dashboard,
  })

  if (isLoading) return <p className="py-16 text-center text-muted">{t('common.loading')}</p>
  if (isError || !data) return <p className="py-16 text-center text-muted">{t('common.error')}</p>

  return (
    <div>
      <h1 className="mb-6 text-2xl font-semibold tracking-tight">{t('dashboard.heading')}</h1>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard icon={<Link2 size={20} />} label={t('dashboard.totalLinks')} value={data.links_total} />
        <StatCard icon={<MousePointerClick size={20} />} label={t('dashboard.totalClicks')} value={data.clicks_total} />
        <StatCard icon={<Zap size={20} />} label={t('dashboard.clicksToday')} value={data.clicks_today} />
      </div>

      <Card className="mt-6 p-6">
        <h2 className="mb-4 text-sm font-medium text-muted">{t('dashboard.trend')}</h2>
        {data.daily.length === 0 ? (
          <p className="py-12 text-center text-sm text-muted">{t('dashboard.empty')}</p>
        ) : (
          <div className="h-64 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={data.daily} margin={{ top: 4, right: 8, left: -22, bottom: 0 }}>
                <defs>
                  <linearGradient id="clickFill" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#f59e0b" stopOpacity={0.35} />
                    <stop offset="100%" stopColor="#f59e0b" stopOpacity={0.02} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="currentColor" opacity={0.08} vertical={false} />
                <XAxis
                  dataKey="date"
                  tick={{ fontSize: 11, fill: '#86868b' }}
                  tickLine={false}
                  axisLine={false}
                  // Axis ticks carry month/day only (e.g. 08/15); the tooltip
                  // still shows the full server-side yyyy-MM-dd date.
                  tickFormatter={(d: string) => d.slice(5).replace('-', '/')}
                />
                <YAxis allowDecimals={false} tick={{ fontSize: 11, fill: '#86868b' }} tickLine={false} axisLine={false} />
                <Tooltip
                  contentStyle={{
                    borderRadius: 12,
                    border: '1px solid var(--color-hairline)',
                    background: 'var(--tooltip-bg)',
                    color: 'var(--tooltip-text)',
                    backdropFilter: 'blur(12px)',
                    boxShadow: '0 8px 30px rgba(0,0,0,0.12)',
                  }}
                  formatter={(value) => [value, t('dashboard.clicks')]}
                />
                <Area
                  type="monotone"
                  dataKey="count"
                  stroke="#f59e0b"
                  strokeWidth={2}
                  fill="url(#clickFill)"
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        )}
      </Card>
    </div>
  )
}
