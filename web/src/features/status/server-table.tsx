import { ChevronDown, ChevronUp } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router-dom'

import { Flag } from '@/components/flag'
import { AvailabilityText, ExpiryText, OnlineDot } from '@/components/status-bits'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { UsageBar } from '@/components/usage-bar'
import { useLang } from '@/i18n/use-lang'
import { daysUntil, formatBytes, formatDuration, formatPercent, formatSpeed } from '@/lib/format'
import { cpuPct, diskPct, memPct, osLabel, trafficPct } from '@/lib/server'
import type { ServerView } from '@/lib/types'
import { cn } from '@/lib/utils'

import { sortBy, type SortKey } from './filters'
import { defaultDesc, type SortState } from './sort-state'

function BarCell({ value, text }: { value: number | null; text?: string }) {
  return (
    <TableCell className="w-28">
      <div className="text-xs tabular">{text ?? (value == null ? '—' : formatPercent(value))}</div>
      {value != null && <UsageBar value={value} className="mt-1" />}
    </TableCell>
  )
}

/** ServerTable 首页列表视图：表头点击排序（与外部排序状态共享）；名称为进入详情的链接（键盘可达），整行点击同样进入详情 */
export function ServerTable({ servers, now, sort, onSort }: { servers: ServerView[]; now: number; sort: SortState; onSort: (s: SortState) => void }) {
  const { t } = useTranslation()
  const lang = useLang()
  const navigate = useNavigate()
  const rows = useMemo(() => (sort ? sortBy(servers, sort.key, sort.desc) : servers), [servers, sort])

  const head = (key: SortKey, label: string, className?: string) => (
    <TableHead className={className}>
      <button
        type="button"
        className="inline-flex items-center gap-1"
        onClick={() => onSort(sort?.key === key ? { key, desc: !sort.desc } : { key, desc: defaultDesc(key) })}
      >
        {label}
        {sort?.key === key && (sort.desc ? <ChevronDown className="size-3" /> : <ChevronUp className="size-3" />)}
      </button>
    </TableHead>
  )

  return (
    <div className="overflow-x-auto rounded-lg border bg-card">
      <Table>
        <TableHeader>
          <TableRow>
            {head('name', t('table.name'))}
            <TableHead className="hidden xl:table-cell">{t('table.system')}</TableHead>
            {head('cpu', 'CPU')}
            {head('mem', t('table.mem'))}
            {head('disk', t('table.disk'), 'hidden sm:table-cell')}
            {head('traffic', t('table.traffic'), 'hidden sm:table-cell')}
            {head('speed', t('table.speed'), 'hidden md:table-cell')}
            {head('uptime', t('table.uptime'), 'hidden lg:table-cell')}
            {head('availability', t('table.availability'), 'hidden lg:table-cell')}
            {head('expire', t('table.expire'), 'hidden lg:table-cell')}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((s) => {
            const m = s.metrics
            const tp = trafficPct(s.traffic)
            const to = `/server/${s.id}`
            return (
              <TableRow key={s.id} className={cn('cursor-pointer', !s.online && 'opacity-60')} onClick={() => navigate(to)}>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <OnlineDot online={s.online} />
                    <Link to={to} className="font-medium hover:underline" onClick={(e) => e.stopPropagation()}>
                      {s.name}
                    </Link>
                    <Flag code={s.country} />
                  </div>
                </TableCell>
                <TableCell className="hidden text-xs text-muted-foreground xl:table-cell">{osLabel(s)}</TableCell>
                <BarCell value={m ? cpuPct(s) : null} />
                <BarCell value={m ? memPct(s) : null} />
                <TableCell className="hidden w-28 sm:table-cell">
                  <div className="text-xs tabular">{m ? formatPercent(diskPct(s)) : '—'}</div>
                  {m && <UsageBar value={diskPct(s)} className="mt-1" />}
                </TableCell>
                <TableCell className="hidden w-28 sm:table-cell">
                  <div className="text-xs tabular">{tp == null ? formatBytes(s.traffic.used) : formatPercent(tp)}</div>
                  {tp != null && <UsageBar value={tp} className="mt-1" />}
                </TableCell>
                <TableCell className="hidden whitespace-nowrap text-xs tabular md:table-cell">
                  {m && s.online ? `↓ ${formatSpeed(m.net_in_speed)} ↑ ${formatSpeed(m.net_out_speed)}` : '—'}
                </TableCell>
                <TableCell className="hidden text-xs tabular lg:table-cell">{m ? formatDuration(m.uptime, lang) : '—'}</TableCell>
                <TableCell className="hidden text-xs lg:table-cell">{s.uptime_24h == null ? '—' : <AvailabilityText value={s.uptime_24h} />}</TableCell>
                <TableCell className="hidden text-xs lg:table-cell">
                  <ExpiryText days={daysUntil(s.expire_at, now)} />
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
