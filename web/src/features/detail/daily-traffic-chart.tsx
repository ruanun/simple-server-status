import { useTranslation } from 'react-i18next'
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

import { formatBytes, formatMonthDay } from '@/lib/format'
import type { DailyTraffic } from '@/lib/types'

const AXIS_TICK = { fontSize: 11, fill: 'var(--muted-foreground)' }

/** DailyTrafficChart 本计费周期每日流量，入站与出站灰阶堆叠（懒加载，避免首页引入图表库） */
export default function DailyTrafficChart({ daily }: { daily: DailyTraffic[] }) {
  const { t } = useTranslation()
  return (
    <div className="h-40">
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={daily} margin={{ top: 4, right: 4, left: 0, bottom: 0 }}>
          <CartesianGrid vertical={false} stroke="var(--border)" />
          <XAxis dataKey="day" tickFormatter={formatMonthDay} tick={AXIS_TICK} tickLine={false} axisLine={false} minTickGap={16} />
          <YAxis tickFormatter={(v: number) => formatBytes(v)} tick={AXIS_TICK} tickLine={false} axisLine={false} width={64} />
          <Tooltip
            labelFormatter={(v) => formatMonthDay(String(v))}
            formatter={(v) => formatBytes(Number(v))}
            contentStyle={{ background: 'var(--popover)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12 }}
          />
          <Bar dataKey="in" name={t('detail.inbound')} stackId="t" fill="var(--foreground)" isAnimationActive={false} />
          <Bar dataKey="out" name={t('detail.outbound')} stackId="t" fill="var(--muted-foreground)" isAnimationActive={false} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}
