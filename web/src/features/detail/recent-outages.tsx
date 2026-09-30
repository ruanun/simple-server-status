import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { useLang } from '@/i18n/use-lang'
import { api, errorMessage } from '@/lib/api'
import { formatDuration, formatHM, formatMDHM } from '@/lib/format'
import type { PublicOutage } from '@/lib/types'

function sameDay(a: number, b: number) {
  return new Date(a * 1000).toDateString() === new Date(b * 1000).toDateString()
}

/** RecentOutages 详情页最近 10 次离线（时间以面板记录为准） */
export function RecentOutages({ serverId }: { serverId: string }) {
  const { t } = useTranslation()
  const lang = useLang()
  const q = useQuery({
    queryKey: ['outages', serverId],
    queryFn: () => api.get<PublicOutage[]>(`/api/public/servers/${encodeURIComponent(serverId)}/outages`),
    refetchInterval: 60_000,
  })
  const line = (o: PublicOutage) => {
    const start = formatMDHM(o.start_at)
    if (o.end_at == null) return `${start} · ${t('detail.outageOngoing')}`
    const end = sameDay(o.start_at, o.end_at) ? formatHM(o.end_at) : formatMDHM(o.end_at)
    return `${start} – ${end} · ${formatDuration(o.duration, lang)}`
  }
  return (
    <section className="space-y-2 rounded-lg border bg-card p-4">
      <h2 className="text-sm font-medium">{t('detail.outages')}</h2>
      {q.data ? (
        q.data.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t('detail.noOutages')}</p>
        ) : (
          <ul className="space-y-1 text-sm tabular">
            {q.data.map((o) => (
              <li key={o.start_at} className={o.end_at == null ? 'text-bad' : undefined}>
                {line(o)}
              </li>
            ))}
          </ul>
        )
      ) : q.error ? (
        <p className="text-sm text-bad">{errorMessage(q.error)}</p>
      ) : (
        <Skeleton className="h-16 w-full" />
      )}
      <p className="text-xs text-muted-foreground">{t('detail.outagesNote')}</p>
    </section>
  )
}
