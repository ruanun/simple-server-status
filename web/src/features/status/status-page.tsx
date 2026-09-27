import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConnectionBanner } from '@/components/connection-banner'
import { SiteHeader } from '@/components/site-header'
import { Skeleton } from '@/components/ui/skeleton'
import { useNow } from '@/hooks/use-now'
import { errorMessage } from '@/lib/api'
import { useLiveServers } from '@/realtime/use-live-servers'

import { FilterBar, type ViewMode } from './filter-bar'
import { applyFilter, type Filter } from './filters'
import { ServerCard } from './server-card'
import { ServerTable } from './server-table'
import { SummaryCards } from './summary-cards'

const VIEW_KEY = 'sss.view'

export function StatusPage() {
  const { t } = useTranslation()
  const { servers, isLoading, error, status } = useLiveServers()
  const now = useNow()
  const [filter, setFilter] = useState<Filter>({ kind: 'all' })
  const [query, setQuery] = useState('')
  const [view, setView] = useState<ViewMode>(() => (localStorage.getItem(VIEW_KEY) === 'list' ? 'list' : 'card'))
  const visible = useMemo(() => applyFilter(servers, filter, query), [servers, filter, query])

  const changeView = (v: ViewMode) => {
    localStorage.setItem(VIEW_KEY, v)
    setView(v)
  }

  let content
  if (isLoading) {
    content = (
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
        {[0, 1, 2].map((i) => (
          <Skeleton key={i} className="h-52 rounded-lg" />
        ))}
      </div>
    )
  } else if (error && servers.length === 0) {
    // 已有数据时后续刷新失败不清空列表，连接状态由提示条表示
    content = <p className="text-sm text-bad">{errorMessage(error)}</p>
  } else if (visible.length === 0) {
    content = <p className="py-16 text-center text-sm text-muted-foreground">{servers.length ? t('empty.noMatch') : t('empty.servers')}</p>
  } else if (view === 'card') {
    content = (
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
        {visible.map((s) => (
          <ServerCard key={s.id} server={s} now={now} />
        ))}
      </div>
    )
  } else {
    content = <ServerTable servers={visible} now={now} />
  }

  return (
    <>
      <SiteHeader />
      {status === 'polling' && <ConnectionBanner />}
      <main className="mx-auto max-w-[1400px] space-y-4 px-4 py-6">
        <SummaryCards servers={servers} />
        <FilterBar servers={servers} filter={filter} onFilter={setFilter} query={query} onQuery={setQuery} view={view} onView={changeView} />
        {content}
      </main>
    </>
  )
}
