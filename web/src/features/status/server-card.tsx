import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'

import { Flag } from '@/components/flag'
import { AvailabilityText, ExpiryText, IpBadges, StatusPill } from '@/components/status-bits'
import { UsageBar } from '@/components/usage-bar'
import { useLang } from '@/i18n/use-lang'
import { daysUntil, formatAgo, formatBytes, formatPercent, formatSpeed } from '@/lib/format'
import { cpuPct, diskPct, memPct, osLabel, trafficPct } from '@/lib/server'
import type { ServerView } from '@/lib/types'
import { cn } from '@/lib/utils'

function Metric({ label, extra, value, valueText, detail }: { label: string; extra?: string; value: number | null; valueText?: string; detail: string }) {
  return (
    <div className="min-w-0">
      <div className="flex items-baseline justify-between gap-2 text-xs">
        <span className="truncate">
          {label}
          {extra && <span className="ml-1 text-muted-foreground">{extra}</span>}
        </span>
        <span className="font-medium tabular">{valueText ?? (value == null ? '—' : formatPercent(value))}</span>
      </div>
      <UsageBar value={value} className="my-1.5" />
      <div className="truncate text-[11px] text-muted-foreground tabular">{detail || ' '}</div>
    </div>
  )
}

/** ServerCard 首页卡片：头部状态、2×2 指标、底部网速与累计流量 */
export function ServerCard({ server: s, now }: { server: ServerView; now: number }) {
  const { t } = useTranslation()
  const lang = useLang()
  const m = s.metrics
  const tp = trafficPct(s.traffic)

  return (
    <Link
      to={`/server/${s.id}`}
      className={cn('block rounded-lg border bg-card p-4 text-card-foreground transition-colors hover:border-foreground/25', !s.online && 'opacity-60')}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className="truncate font-semibold">{s.name}</span>
          <Flag code={s.country} />
          <IpBadges v4={s.ipv4} v6={s.ipv6} />
        </div>
        <StatusPill server={s} />
      </div>
      <div className="mt-1 flex justify-between gap-2 text-xs text-muted-foreground">
        <span className="truncate">{osLabel(s)}</span>
        <span className="flex shrink-0 items-center gap-1.5">
          <AvailabilityText value={s.uptime_24h} label={t('status.availability')} />
          {s.uptime_24h != null && <span aria-hidden>·</span>}
          <ExpiryText days={daysUntil(s.expire_at, now)} />
        </span>
      </div>

      <div className="mt-4 grid grid-cols-2 gap-x-4 gap-y-3">
        <Metric
          label="CPU"
          extra={s.static?.cpu_cores ? t('metric.cores', { count: s.static.cpu_cores }) : undefined}
          value={m ? cpuPct(s) : null}
          detail={m ? `load ${m.load1.toFixed(2)} ${m.load5.toFixed(2)} ${m.load15.toFixed(2)}` : ''}
        />
        <Metric
          label={t('metric.mem')}
          value={m ? memPct(s) : null}
          detail={m && s.static ? `${formatBytes(m.mem_used)} / ${formatBytes(s.static.mem_total)}` : ''}
        />
        <Metric label={t('metric.disk')} value={m ? diskPct(s) : null} detail={m ? `${formatBytes(m.disk_used)} / ${formatBytes(m.disk_total)}` : ''} />
        <Metric
          label={t('metric.traffic')}
          value={tp}
          valueText={tp == null ? '∞' : undefined}
          detail={s.traffic.limit ? `${formatBytes(s.traffic.used)} / ${formatBytes(s.traffic.limit)}` : t('metric.thisMonth', { value: formatBytes(s.traffic.used) })}
        />
      </div>

      <div className="mt-3 flex justify-between gap-2 border-t pt-2.5 text-xs tabular">
        {s.online && m ? (
          <span>
            ↓ {formatSpeed(m.net_in_speed)} ↑ {formatSpeed(m.net_out_speed)}
          </span>
        ) : (
          <span className="text-muted-foreground">{s.last_seen ? t('status.lastSeen', { ago: formatAgo(s.last_seen, now, lang) }) : t('status.neverSeen')}</span>
        )}
        {m && (
          <span className="truncate text-muted-foreground">
            ↓ {formatBytes(m.net_in_total)} ↑ {formatBytes(m.net_out_total)}
          </span>
        )}
      </div>
    </Link>
  )
}
