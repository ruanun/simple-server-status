import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { ExpiryText } from '@/components/status-bits'
import type { Lang } from '@/lib/format'
import { daysUntil, formatBytes, formatDate } from '@/lib/format'
import type { ServerView } from '@/lib/types'

interface Item {
  label: string
  value: ReactNode
  sub?: ReactNode
}

/** InfoGrid 详情页顶部的系统、硬件、流量与续费信息 */
export function InfoGrid({ server: s, now, lang }: { server: ServerView; now: number; lang: Lang }) {
  const { t } = useTranslation()
  const st = s.static
  const days = daysUntil(s.expire_at, now)
  const expireDate = s.expire_at ? t('detail.expireAt', { date: formatDate(s.expire_at, lang) }) : undefined
  const items: Item[] = [
    { label: t('detail.system'), value: st ? `${st.platform || st.os} ${st.platform_version}`.trim() : '—', sub: st?.kernel },
    {
      label: t('detail.cpu'),
      value: st?.cpu_model ? `${st.cpu_model} × ${st.cpu_cores}` : '—',
      sub: s.metrics ? t('detail.cpuSub', { load: s.metrics.load1.toFixed(2), procs: s.metrics.procs, tcp: s.metrics.tcp }) : undefined,
    },
    { label: t('detail.memory'), value: st ? `${formatBytes(st.mem_total)} / ${formatBytes(st.disk_total)} / ${formatBytes(st.swap_total)}` : '—' },
    { label: t('detail.arch'), value: st ? [st.arch, st.virtualization].filter(Boolean).join(' · ') || '—' : '—' },
    {
      label: t('detail.traffic'),
      value: `${formatBytes(s.traffic.used)}${s.traffic.limit ? ` / ${formatBytes(s.traffic.limit)}` : ''}`,
      sub: t('detail.resetDay', { day: s.traffic.reset_day }),
    },
    {
      label: t('detail.renew'),
      // 价格显示为「币种 金额 / 周期」；设置了价格时到期提醒移到副行，仍保留颜色提示与日期
      value:
        s.price != null ? (
          `${s.currency ? `${s.currency} ` : ''}${s.price} / ${s.billing_cycle ? t(`billing.${s.billing_cycle}`) : '—'}`
        ) : (
          <ExpiryText days={days} />
        ),
      sub:
        s.price != null ? (
          <>
            <ExpiryText days={days} />
            {expireDate && ` · ${expireDate}`}
          </>
        ) : (
          expireDate
        ),
    },
  ]
  return (
    <div className="grid gap-4 rounded-lg border bg-card p-4 sm:grid-cols-2 lg:grid-cols-3">
      {items.map((i) => (
        <div key={i.label} className="min-w-0">
          <div className="text-xs text-muted-foreground">{i.label}</div>
          <div className="truncate text-sm font-medium tabular">{i.value}</div>
          {i.sub && <div className="truncate text-xs text-muted-foreground">{i.sub}</div>}
        </div>
      ))}
    </div>
  )
}
