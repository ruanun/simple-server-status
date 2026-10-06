import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Fragment, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router-dom'

import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useLang } from '@/i18n/use-lang'
import { adminApi, adminKeys } from '@/lib/admin-api'
import { errorMessage } from '@/lib/api'
import { formatDateTime, formatDuration } from '@/lib/format'
import type { AdminEvent, EventKind, NotifyLogStatus } from '@/lib/types'
import { cn } from '@/lib/utils'

const PAGE_SIZE = 50
const ALL = '__all'

// 类型筛选：网址中的 type 对应接口的 kind（负载合为一项）
const TYPE_FILTERS = { offline: 'offline', load: 'load_cpu,load_mem,load_disk', reboot: 'reboot', ip_change: 'ip_change' } as const
type TypeFilter = keyof typeof TYPE_FILTERS

function isTypeFilter(v: string): v is TypeFilter {
  return v in TYPE_FILTERS
}

const INSTANT: EventKind[] = ['reboot', 'ip_change']

const KIND_DOT: Record<EventKind, string> = {
  offline: 'bg-bad',
  load_cpu: 'bg-warn',
  load_mem: 'bg-warn',
  load_disk: 'bg-warn',
  reboot: 'bg-muted-foreground',
  ip_change: 'bg-muted-foreground',
}

/** EventDetailText 按类型格式化事件详情 */
function EventDetailText({ e }: { e: AdminEvent }) {
  const { t } = useTranslation()
  const d = e.detail
  switch (e.kind) {
    case 'load_cpu':
    case 'load_mem':
    case 'load_disk':
      return t('events.loadDetail', { peak: (d.peak ?? 0).toFixed(1), threshold: d.threshold, minutes: d.minutes })
    case 'ip_change':
      return (['ipv4', 'ipv6'] as const)
        .flatMap((k) => {
          const v = d[k]
          return v ? [`${k === 'ipv4' ? 'IPv4' : 'IPv6'} ${v[0]} → ${v[1]}`] : []
        })
        .join('; ')
    default:
      return ''
  }
}

function Pager({ page, total, onPage }: { page: number; total: number; onPage: (p: number) => void }) {
  const { t } = useTranslation()
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  return (
    <div className="flex items-center justify-end gap-2 text-sm">
      <span className="text-muted-foreground tabular">{t('events.page', { page, pages })}</span>
      <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => onPage(page - 1)}>
        {t('events.prev')}
      </Button>
      <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => onPage(page + 1)}>
        {t('events.next')}
      </Button>
    </div>
  )
}

/** EventsPage 后台事件：事件与通知记录，筛选与分页写入网址 */
export function EventsPage() {
  const { t } = useTranslation()
  const lang = useLang()
  const [params, setParams] = useSearchParams()
  const tab = params.get('tab') === 'notify' ? 'notify' : 'events'
  const server = params.get('server') ?? ''
  const status = params.get('status') ?? ''
  const rawType = params.get('type') ?? ''
  const type = isTypeFilter(rawType) ? rawType : ''
  const page = Math.max(1, Number(params.get('page')) || 1)
  const [open, setOpen] = useState<number | null>(null)

  const update = (patch: Record<string, string>) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        for (const [k, v] of Object.entries(patch)) {
          if (v) next.set(k, v)
          else next.delete(k)
        }
        return next
      },
      { replace: true },
    )

  const servers = useQuery({ queryKey: adminKeys.servers, queryFn: adminApi.servers })
  const events = useQuery({
    queryKey: ['admin', 'events', server, type, page],
    queryFn: () => adminApi.events({ server, kind: type ? TYPE_FILTERS[type] : '', page }),
    enabled: tab === 'events',
    placeholderData: keepPreviousData,
  })
  const logs = useQuery({
    queryKey: ['admin', 'notify-log', server, status, page],
    queryFn: () => adminApi.notifyLog({ server, status, page }),
    enabled: tab === 'notify',
    placeholderData: keepPreviousData,
  })
  const q = tab === 'events' ? events : logs

  const statusText: Record<NotifyLogStatus, string> = { sent: t('events.sent'), failed: t('events.failed'), pending: t('events.pending') }

  return (
    <div className="space-y-4">
      <h1 className="text-lg font-semibold">{t('events.title')}</h1>
      <Tabs value={tab} onValueChange={(v) => update({ tab: v === 'events' ? '' : v, page: '' })}>
        <TabsList>
          <TabsTrigger value="events">{t('events.list')}</TabsTrigger>
          <TabsTrigger value="notify">{t('events.notify')}</TabsTrigger>
        </TabsList>
      </Tabs>
      <div className="flex flex-wrap gap-2">
        <Select value={server || ALL} onValueChange={(v) => update({ server: v === ALL ? '' : v, page: '' })}>
          <SelectTrigger className="w-44" aria-label={t('events.server')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>{t('events.allServers')}</SelectItem>
            {(servers.data ?? []).map((s) => (
              <SelectItem key={s.id} value={s.id}>
                {s.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {tab === 'events' && (
          <Select value={type || ALL} onValueChange={(v) => update({ type: v === ALL ? '' : v, page: '' })}>
            <SelectTrigger className="w-32" aria-label={t('events.kind')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>{t('events.allTypes')}</SelectItem>
              {(Object.keys(TYPE_FILTERS) as TypeFilter[]).map((k) => (
                <SelectItem key={k} value={k}>
                  {t(`events.filters.${k}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
        {tab === 'notify' && (
          <Select value={status || ALL} onValueChange={(v) => update({ status: v === ALL ? '' : v, page: '' })}>
            <SelectTrigger className="w-32" aria-label={t('events.status')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>{t('events.allStatus')}</SelectItem>
              <SelectItem value="sent">{statusText.sent}</SelectItem>
              <SelectItem value="failed">{statusText.failed}</SelectItem>
              <SelectItem value="pending">{statusText.pending}</SelectItem>
            </SelectContent>
          </Select>
        )}
      </div>
      {q.error ? (
        <p className="text-sm text-bad">{errorMessage(q.error)}</p>
      ) : !q.data ? (
        <Skeleton className="h-48" />
      ) : q.data.items.length === 0 ? (
        <p className="rounded-lg border bg-card py-12 text-center text-sm text-muted-foreground">{t('events.empty')}</p>
      ) : (
        <div className="overflow-x-auto rounded-lg border bg-card">
          {tab === 'events' && events.data ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('events.server')}</TableHead>
                  <TableHead>{t('events.kind')}</TableHead>
                  <TableHead>{t('events.start')}</TableHead>
                  <TableHead className="hidden sm:table-cell">{t('events.end')}</TableHead>
                  <TableHead>{t('events.duration')}</TableHead>
                  <TableHead className="hidden md:table-cell">{t('events.detail')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {events.data.items.map((e) => {
                  const instant = INSTANT.includes(e.kind)
                  return (
                    <TableRow key={e.id}>
                      <TableCell>{e.server_name || e.server_id}</TableCell>
                      <TableCell className="text-xs">
                        <span className="inline-flex items-center gap-1.5 whitespace-nowrap">
                          <span className={cn('size-1.5 rounded-full', KIND_DOT[e.kind])} />
                          {t(`events.types.${e.kind}`, { defaultValue: e.kind })}
                        </span>
                      </TableCell>
                      <TableCell className="text-xs tabular">{formatDateTime(e.start_at, lang)}</TableCell>
                      <TableCell className="hidden text-xs tabular sm:table-cell">
                        {instant || e.end_at == null ? '—' : formatDateTime(e.end_at, lang)}
                      </TableCell>
                      <TableCell className="text-xs tabular">
                        {instant ? (
                          '—'
                        ) : (
                          <>
                            {e.end_at == null && <span className="mr-1 text-bad">{t('events.ongoing')}</span>}
                            {formatDuration(e.duration, lang)}
                          </>
                        )}
                      </TableCell>
                      <TableCell className="hidden text-xs text-muted-foreground md:table-cell"><EventDetailText e={e} /></TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          ) : logs.data ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('events.time')}</TableHead>
                  <TableHead>{t('events.server')}</TableHead>
                  <TableHead className="hidden sm:table-cell">{t('events.kind')}</TableHead>
                  <TableHead className="hidden md:table-cell">{t('events.channel')}</TableHead>
                  <TableHead>{t('events.status')}</TableHead>
                  <TableHead className="hidden md:table-cell">{t('events.error')}</TableHead>
                  <TableHead className="w-10" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.data.items.map((l) => (
                  <Fragment key={l.id}>
                    <TableRow>
                      <TableCell className="text-xs tabular">{formatDateTime(l.created_at, lang)}</TableCell>
                      <TableCell>{l.server_id ? l.server_name || l.server_id : t('events.test')}</TableCell>
                      <TableCell className="hidden text-xs sm:table-cell">{l.kind === 'test' ? '—' : t(`events.kinds.${l.kind}`, { defaultValue: l.kind })}</TableCell>
                      <TableCell className="hidden text-xs md:table-cell">{l.channel === 'telegram' ? 'Telegram' : 'Webhook'}</TableCell>
                      <TableCell className={cn('text-xs', l.status === 'failed' && 'text-bad')}>{statusText[l.status]}</TableCell>
                      <TableCell className="hidden max-w-60 truncate text-xs text-bad md:table-cell">{l.error}</TableCell>
                      <TableCell>
                        <Button variant="ghost" size="sm" aria-expanded={open === l.id} onClick={() => setOpen(open === l.id ? null : l.id)}>
                          {open === l.id ? t('events.collapse') : t('events.expand')}
                        </Button>
                      </TableCell>
                    </TableRow>
                    {open === l.id && (
                      <TableRow>
                        <TableCell colSpan={7} className="space-y-1 text-xs text-muted-foreground">
                          <p className="font-medium text-foreground">{l.title}</p>
                          <p className="whitespace-pre-line">{l.message}</p>
                          {l.error && <p className="text-bad">{l.error}</p>}
                        </TableCell>
                      </TableRow>
                    )}
                  </Fragment>
                ))}
              </TableBody>
            </Table>
          ) : null}
        </div>
      )}
      {q.data && <Pager page={page} total={q.data.total} onPage={(p) => update({ page: p > 1 ? String(p) : '' })} />}
    </div>
  )
}
