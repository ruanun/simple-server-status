import { useTranslation } from 'react-i18next'

import { useLang } from '@/i18n/use-lang'
import { formatAgo, formatAvailability, formatDateTime, formatDuration } from '@/lib/format'
import { availabilityLevel, expiryLevel } from '@/lib/server'
import type { ServerView } from '@/lib/types'
import { cn } from '@/lib/utils'

export function OnlineDot({ online }: { online: boolean }) {
  return <span className={cn('inline-block size-1.5 shrink-0 rounded-full', online ? 'bg-online' : 'bg-bad')} />
}

/** StatusPill 在线显示运行时长，离线显示红色标记 */
export function StatusPill({ server }: { server: ServerView }) {
  const { t } = useTranslation()
  const lang = useLang()
  if (!server.online) {
    return (
      <span className="inline-flex shrink-0 items-center gap-1 rounded-full border border-bad/30 px-2 py-0.5 text-[11px] text-bad">
        <OnlineDot online={false} />
        {t('status.offline')}
      </span>
    )
  }
  return (
    <span className="inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 text-[11px]">
      <OnlineDot online />
      {server.metrics ? t('status.onlineFor', { duration: formatDuration(server.metrics.uptime, lang) }) : t('status.online')}
    </span>
  )
}

/** ExpiryText 到期提示：长期 / N 天后到期（≤7 天琥珀）/ 已过期（红） */
export function ExpiryText({ days }: { days: number | null }) {
  const { t } = useTranslation()
  if (days == null) return <span>{t('expire.never')}</span>
  const level = expiryLevel(days)
  return (
    <span className={cn(level === 'warn' && 'text-warn', level === 'bad' && 'text-bad')}>
      {days <= 0 ? t('expire.expired') : t('expire.days', { count: days })}
    </span>
  )
}

/** AvailabilityText 24 小时在线率；无数据时不显示 */
export function AvailabilityText({ value, label }: { value: number | null | undefined; label?: string }) {
  if (value == null) return null
  const level = availabilityLevel(value)
  return (
    <span className={cn('tabular', level === 'warn' && 'text-warn', level === 'bad' && 'text-bad')}>
      {label ? `${label} ` : ''}
      {formatAvailability(value)}
    </span>
  )
}

/** IpBadges IPv4 / IPv6 支持标记；都不支持时不显示 */
export function IpBadges({ v4, v6 }: { v4?: boolean; v6?: boolean }) {
  const { t } = useTranslation()
  if (!v4 && !v6) return null
  const badge = (label: string, sr: string) => (
    <span className="rounded border px-1 text-[10px] leading-4 text-muted-foreground" title={sr}>
      <span aria-hidden>{label}</span>
      <span className="sr-only">{sr}</span>
    </span>
  )
  return (
    <span className="inline-flex shrink-0 gap-1">
      {v4 && badge('v4', t('status.ipv4'))}
      {v6 && badge('v6', t('status.ipv6'))}
    </span>
  )
}

/** LastReport 最后上报的相对时间，悬停显示具体时间；从未上报时显示「尚未上报」 */
export function LastReport({ ts, now }: { ts: number; now: number }) {
  const { t } = useTranslation()
  const lang = useLang()
  if (!ts) return <span>{t('status.neverSeen')}</span>
  return (
    <time dateTime={new Date(ts * 1000).toISOString()} title={formatDateTime(ts, lang)}>
      {formatAgo(ts, now, lang)}
    </time>
  )
}
