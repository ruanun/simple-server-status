import { useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { lazy, Suspense, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useParams } from 'react-router-dom'

import { Flag } from '@/components/flag'
import { SiteHeader } from '@/components/site-header'
import { IpBadges, LastReport, StatusPill } from '@/components/status-bits'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useNow } from '@/hooks/use-now'
import { useLang } from '@/i18n/use-lang'
import { api, errorMessage } from '@/lib/api'
import { formatAvailability, formatBytes, formatMonthDay } from '@/lib/format'
import { RANGES, type Point, type Range, type ServerStats } from '@/lib/types'
import { useLiveServers } from '@/realtime/use-live-servers'

import { DiskList } from './disk-list'
import { InfoGrid } from './info-grid'
import { RecentOutages } from './recent-outages'

const MetricCharts = lazy(() => import('./metric-charts'))
const DailyTrafficChart = lazy(() => import('./daily-traffic-chart'))

export function DetailPage() {
  const { id = '' } = useParams()
  const { t } = useTranslation()
  const lang = useLang()
  const now = useNow()
  const { servers, isLoading, error } = useLiveServers()
  const [range, setRange] = useState<Range>('realtime')
  const server = servers.find((s) => s.id === id)

  const points = useQuery({
    queryKey: ['metrics', id, range],
    queryFn: () => api.get<Point[]>(`/api/public/servers/${encodeURIComponent(id)}/metrics?range=${range}`),
    enabled: server !== undefined,
    refetchInterval: range === 'realtime' ? 2000 : 60_000,
  })

  const stats = useQuery({
    queryKey: ['stats', id],
    queryFn: () => api.get<ServerStats>(`/api/public/servers/${encodeURIComponent(id)}/stats`),
    enabled: server !== undefined,
    refetchInterval: 300_000,
  })

  let body
  if (isLoading) {
    body = <Skeleton className="h-48 w-full rounded-lg" />
  } else if (!server && error) {
    // 列表请求失败（网络或服务端错误）时显示错误原因，而不是误报服务器不存在
    body = <p className="py-16 text-center text-sm text-bad">{errorMessage(error)}</p>
  } else if (!server) {
    body = <p className="py-16 text-center text-sm text-muted-foreground">{t('detail.notFound')}</p>
  } else {
    body = (
      <>
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="text-xl font-semibold tracking-tight">{server.name}</h1>
          <Flag code={server.country} />
          <IpBadges v4={server.ipv4} v6={server.ipv6} />
          <StatusPill server={server} />
          <span className="text-xs text-muted-foreground tabular">
            {t('detail.lastSeen')} <LastReport ts={server.last_seen} now={now} />
          </span>
          {stats.data && (stats.data.uptime_24h != null || stats.data.uptime_7d != null) && (
            <span className="text-xs text-muted-foreground tabular">
              {t('detail.availability')} 24h {stats.data.uptime_24h == null ? '—' : formatAvailability(stats.data.uptime_24h)} · 7d{' '}
              {stats.data.uptime_7d == null ? '—' : formatAvailability(stats.data.uptime_7d)}
            </span>
          )}
        </div>
        <InfoGrid server={server} now={now} lang={lang} />
        {server.metrics && server.metrics.disks.length > 0 && <DiskList disks={server.metrics.disks} />}
        <section className="space-y-3 rounded-lg border bg-card p-4">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <h2 className="text-sm font-medium">
              {t('detail.dailyTraffic')}
              {stats.data && ` ${formatMonthDay(stats.data.period_start)} – ${formatMonthDay(stats.data.period_end)}`}
            </h2>
            {server.traffic.limit != null && (
              <span className="text-xs text-muted-foreground tabular">
                {t('detail.trafficUsedOf', { used: formatBytes(server.traffic.used), limit: formatBytes(server.traffic.limit) })}
              </span>
            )}
          </div>
          {stats.data ? (
            <Suspense fallback={<Skeleton className="h-40 w-full" />}>
              <DailyTrafficChart daily={stats.data.daily} />
            </Suspense>
          ) : stats.error ? (
            <p className="py-12 text-center text-sm text-bad">{errorMessage(stats.error)}</p>
          ) : (
            <Skeleton className="h-40 w-full" />
          )}
        </section>
        <RecentOutages serverId={server.id} />
        <section className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-sm font-medium">{t('detail.history')}</h2>
            <Tabs value={range} onValueChange={(v) => setRange(v as Range)}>
              <TabsList>
                {RANGES.map((r) => (
                  <TabsTrigger key={r} value={r}>
                    {t(`range.${r}`)}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
          </div>
          {points.data && points.data.length > 0 ? (
            <Suspense fallback={<Skeleton className="h-40 w-full rounded-lg" />}>
              <MetricCharts points={points.data} range={range} />
            </Suspense>
          ) : points.error ? (
            <p className="rounded-lg border bg-card py-12 text-center text-sm text-bad">{errorMessage(points.error)}</p>
          ) : (
            <p className="rounded-lg border bg-card py-12 text-center text-sm text-muted-foreground">
              {points.isLoading ? t('common.loading') : t('detail.noData')}
            </p>
          )}
        </section>
      </>
    )
  }

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-[1400px] space-y-4 px-4 py-6">
        <Button asChild variant="ghost" size="sm" className="-ml-2">
          <Link to="/">
            <ArrowLeft className="size-4" />
            {t('nav.back')}
          </Link>
        </Button>
        {body}
      </main>
    </>
  )
}
