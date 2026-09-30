import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ExpiryText } from '@/components/status-bits'
import { adminApi, adminKeys } from '@/lib/admin-api'
import { errorMessage } from '@/lib/api'

/** OverviewBar 后台总览：按币种的月均费用与 30 天内到期的服务器 */
export function OverviewBar() {
  const { t } = useTranslation()
  const { data, error } = useQuery({ queryKey: adminKeys.overview, queryFn: adminApi.overview, refetchInterval: 60_000 })
  const [open, setOpen] = useState(false)
  if (error && !data) {
    return (
      <div className="rounded-lg border bg-card px-4 py-3">
        <p className="text-sm text-bad">{errorMessage(error)}</p>
      </div>
    )
  }
  if (!data) return null
  const cost = data.monthly_cost.map((g) => `${g.currency ? `${g.currency} ` : ''}${g.amount}`).join(' · ')
  return (
    <div className="space-y-2 rounded-lg border bg-card px-4 py-3 text-sm">
      <div className="flex flex-wrap items-center gap-x-6 gap-y-1">
        <span>
          <span className="text-muted-foreground">{t('overview.monthlyCost')}：</span>
          <span className="tabular">{cost || '—'}</span>
        </span>
        {data.expiring.length > 0 ? (
          <button type="button" className="text-warn underline-offset-2 hover:underline" aria-expanded={open} onClick={() => setOpen((v) => !v)}>
            {t('overview.expiring', { count: data.expiring.length })}
          </button>
        ) : (
          <span>
            <span className="text-muted-foreground">{t('overview.expiringLabel')}：</span>
            <span className="tabular">—</span>
          </span>
        )}
      </div>
      {open && (
        <ul className="space-y-1 text-xs">
          {data.expiring.map((s) => (
            <li key={s.id}>
              {s.name} · <ExpiryText days={s.days} />
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
