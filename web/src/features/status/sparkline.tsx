import type { SpeedSample } from '@/realtime/speed-history'

/** Sparkline 汇总卡中的迷你双线图（下行实线、上行浅色），纯 SVG 避免首页加载图表库 */
export function Sparkline({ samples }: { samples: SpeedSample[] }) {
  if (samples.length < 2) return <div className="h-5" />
  const max = Math.max(1, ...samples.map((s) => Math.max(s.in, s.out)))
  const line = (pick: (s: SpeedSample) => number) =>
    samples.map((s, i) => `${((i / (samples.length - 1)) * 100).toFixed(2)},${(19 - (pick(s) / max) * 18).toFixed(2)}`).join(' ')
  return (
    <svg viewBox="0 0 100 20" preserveAspectRatio="none" className="h-5 w-full" aria-hidden>
      <polyline fill="none" stroke="currentColor" strokeWidth="1.2" vectorEffect="non-scaling-stroke" className="text-foreground" points={line((s) => s.in)} />
      <polyline fill="none" stroke="currentColor" strokeWidth="1.2" vectorEffect="non-scaling-stroke" className="text-muted-foreground/60" points={line((s) => s.out)} />
    </svg>
  )
}
