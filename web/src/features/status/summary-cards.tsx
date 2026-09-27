import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { formatBytes, formatPercent, formatSpeed } from '@/lib/format'
import { cpuPct } from '@/lib/server'
import type { ServerView } from '@/lib/types'
import { useSpeedHistory } from '@/realtime/speed-history'

import { Sparkline } from './sparkline'

function Stat({ label, value, sub }: { label: string; value: ReactNode; sub: ReactNode }) {
  return (
    <div className="min-w-0 rounded-lg border bg-card p-4">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 truncate text-2xl font-semibold tracking-tight tabular">{value}</div>
      <div className="mt-1 truncate text-xs text-muted-foreground tabular">{sub}</div>
    </div>
  )
}

/** SummaryCards 首页四张汇总卡：在线数、最忙节点、本月流量、实时网速 */
export function SummaryCards({ servers }: { servers: ServerView[] }) {
  const { t } = useTranslation()
  const history = useSpeedHistory()
  const online = servers.filter((s) => s.online)
  const offline = servers.length - online.length
  const busiest = online.reduce<ServerView | null>((best, s) => (!best || cpuPct(s) > cpuPct(best) ? s : best), null)
  const traffic = servers.reduce((a, s) => ({ in: a.in + s.traffic.in, out: a.out + s.traffic.out }), { in: 0, out: 0 })
  const speed = online.reduce((a, s) => ({ in: a.in + (s.metrics?.net_in_speed ?? 0), out: a.out + (s.metrics?.net_out_speed ?? 0) }), { in: 0, out: 0 })

  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <Stat
        label={t('summary.nodes')}
        value={
          <>
            {online.length}
            <span className="text-base font-normal text-muted-foreground">/{servers.length}</span>
          </>
        }
        sub={offline > 0 ? t('summary.offline', { count: offline }) : t('summary.allOnline')}
      />
      <Stat label={t('summary.busiest')} value={busiest ? formatPercent(cpuPct(busiest)) : '—'} sub={busiest ? `${busiest.name} · CPU` : '—'} />
      <Stat label={t('summary.traffic')} value={formatBytes(traffic.in + traffic.out)} sub={`↓ ${formatBytes(traffic.in)} ↑ ${formatBytes(traffic.out)}`} />
      <Stat label={t('summary.speed')} value={formatSpeed(speed.in + speed.out)} sub={<Sparkline samples={history} />} />
    </div>
  )
}
