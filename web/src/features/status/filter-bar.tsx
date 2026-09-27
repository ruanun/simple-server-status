import { LayoutGrid, List, Search } from 'lucide-react'
import { useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Flag } from '@/components/flag'
import { Input } from '@/components/ui/input'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import type { ServerView } from '@/lib/types'
import { cn } from '@/lib/utils'

import { buildChips, sameFilter, type Filter } from './filters'

export type ViewMode = 'card' | 'list'

interface Props {
  servers: ServerView[]
  filter: Filter
  onFilter: (f: Filter) => void
  query: string
  onQuery: (q: string) => void
  view: ViewMode
  onView: (v: ViewMode) => void
}

/** FilterBar 筛选标签（全部 / 分组 / 地区 / 离线）、名称搜索与视图切换 */
export function FilterBar({ servers, filter, onFilter, query, onQuery, view, onView }: Props) {
  const { t } = useTranslation()
  const chips = useMemo(() => buildChips(servers), [servers])

  const chip = (key: string, label: ReactNode, count: number, f: Filter) => (
    <button
      key={key}
      type="button"
      aria-pressed={sameFilter(filter, f)}
      onClick={() => onFilter(f)}
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs transition-colors',
        sameFilter(filter, f) ? 'border-foreground bg-foreground text-background' : 'bg-card hover:bg-accent',
      )}
    >
      {label}
      <span className="tabular opacity-60">{count}</span>
    </button>
  )

  return (
    <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
      <div className="flex flex-wrap gap-1.5">
        {chip('all', t('filter.all'), servers.length, { kind: 'all' })}
        {chips.groups.map((g) => chip(`g:${g.value}`, g.value, g.count, { kind: 'group', value: g.value }))}
        {chips.countries.map((c) =>
          chip(
            `c:${c.value}`,
            <>
              <Flag code={c.value} />
              {c.value}
            </>,
            c.count,
            { kind: 'country', value: c.value },
          ),
        )}
        {chips.offline > 0 && chip('offline', t('filter.offline'), chips.offline, { kind: 'offline' })}
      </div>
      <div className="flex items-center gap-2">
        <div className="relative min-w-0 flex-1 md:flex-none">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input value={query} onChange={(e) => onQuery(e.target.value)} placeholder={t('filter.search')} className="h-8 w-full pl-8 text-sm md:w-48" />
        </div>
        <ToggleGroup type="single" variant="outline" size="sm" value={view} onValueChange={(v) => v && onView(v as ViewMode)}>
          <ToggleGroupItem value="card" aria-label={t('view.card')}>
            <LayoutGrid className="size-4" />
          </ToggleGroupItem>
          <ToggleGroupItem value="list" aria-label={t('view.list')}>
            <List className="size-4" />
          </ToggleGroupItem>
        </ToggleGroup>
      </div>
    </div>
  )
}
