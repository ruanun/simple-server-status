import { useTranslation } from 'react-i18next'
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

import { useLang } from '@/i18n/use-lang'
import { formatChartLabel, formatChartTick, formatSpeed } from '@/lib/format'
import type { Point, Range } from '@/lib/types'

type Unit = 'percent' | 'speed' | 'number'

interface Line {
  key: keyof Point
  label: string
  muted?: boolean
  /** right 使用右侧纵轴（量纲不同的第二条线，如 TCP 连接数） */
  right?: boolean
}

const AXIS_TICK = { fontSize: 11, fill: 'var(--muted-foreground)' }

function formatter(unit: Unit): (v: number) => string {
  if (unit === 'percent') return (v) => `${Math.round(v)}%`
  if (unit === 'speed') return formatSpeed
  return (v) => String(Math.round(v * 100) / 100)
}

interface ChartProps {
  title: string
  points: Point[]
  lines: Line[]
  unit: Unit
  tick: (ts: number) => string
  label: (ts: number) => string
}

function ChartCard({ title, points, lines, unit, tick, label }: ChartProps) {
  const fmt = formatter(unit)
  const dual = lines.some((l) => l.right)
  return (
    <div className="rounded-lg border bg-card p-4">
      <div className="mb-2 text-sm font-medium">{title}</div>
      <div className="h-40">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={points} margin={{ top: 4, right: 4, left: 0, bottom: 0 }}>
            <CartesianGrid vertical={false} stroke="var(--border)" yAxisId="left" />
            <XAxis dataKey="ts" tickFormatter={tick} tick={AXIS_TICK} tickLine={false} axisLine={false} minTickGap={48} />
            <YAxis yAxisId="left" tickFormatter={fmt} tick={AXIS_TICK} tickLine={false} axisLine={false} width={64} domain={unit === 'percent' ? [0, 100] : [0, 'auto']} />
            {dual && <YAxis yAxisId="right" orientation="right" tickFormatter={fmt} tick={AXIS_TICK} tickLine={false} axisLine={false} width={48} domain={[0, 'auto']} />}
            <Tooltip
              labelFormatter={(v) => label(Number(v))}
              formatter={(v) => fmt(Number(v))}
              contentStyle={{ background: 'var(--popover)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12 }}
            />
            {lines.map((l) => (
              <Area
                key={l.key}
                yAxisId={l.right ? 'right' : 'left'}
                type="monotone"
                dataKey={l.key}
                name={l.label}
                stroke={l.muted ? 'var(--muted-foreground)' : 'var(--foreground)'}
                fill={l.muted ? 'transparent' : 'var(--muted)'}
                strokeWidth={1.5}
                dot={false}
                isAnimationActive={false}
              />
            ))}
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}

/** MetricCharts 详情页 5 张灰阶历史图（懒加载，避免首页引入图表库） */
export default function MetricCharts({ points, range }: { points: Point[]; range: Range }) {
  const { t } = useTranslation()
  const lang = useLang()
  const tick = (ts: number) => formatChartTick(ts, range, lang)
  const label = (ts: number) => formatChartLabel(ts, lang)
  return (
    <div className="grid gap-3">
      <ChartCard title="CPU" points={points} lines={[{ key: 'cpu', label: 'CPU' }]} unit="percent" tick={tick} label={label} />
      <ChartCard title={t('metric.mem')} points={points} lines={[{ key: 'mem', label: t('metric.mem') }]} unit="percent" tick={tick} label={label} />
      <ChartCard
        title={t('detail.network')}
        points={points}
        lines={[
          { key: 'net_in', label: t('detail.inbound') },
          { key: 'net_out', label: t('detail.outbound'), muted: true },
        ]}
        unit="speed"
        tick={tick}
        label={label}
      />
      <ChartCard title={t('metric.disk')} points={points} lines={[{ key: 'disk', label: t('metric.disk') }]} unit="percent" tick={tick} label={label} />
      <ChartCard
        title={t('detail.loadTcp')}
        points={points}
        lines={[
          { key: 'load1', label: 'load1' },
          { key: 'tcp', label: 'TCP', muted: true, right: true },
        ]}
        unit="number"
        tick={tick}
        label={label}
      />
    </div>
  )
}
