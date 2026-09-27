import { useTranslation } from 'react-i18next'

import { UsageBar } from '@/components/usage-bar'
import { formatBytes, formatPercent, percent } from '@/lib/format'
import type { Disk } from '@/lib/types'

export function DiskList({ disks }: { disks: Disk[] }) {
  const { t } = useTranslation()
  return (
    <div className="rounded-lg border bg-card p-4">
      <div className="mb-3 text-sm font-medium">{t('detail.disks')}</div>
      <div className="grid gap-4 sm:grid-cols-2">
        {disks.map((d) => {
          const p = percent(d.used, d.total)
          return (
            <div key={d.mount} className="min-w-0">
              <div className="flex justify-between gap-2 text-xs">
                <span className="truncate">
                  <span className="font-medium">{d.mount}</span> <span className="text-muted-foreground">{d.fstype}</span>
                </span>
                <span className="tabular">{formatPercent(p)}</span>
              </div>
              <UsageBar value={p} className="my-1.5" />
              <div className="text-[11px] text-muted-foreground tabular">
                {formatBytes(d.used)} / {formatBytes(d.total)}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
