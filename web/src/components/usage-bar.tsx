import { usageLevel, type Level } from '@/lib/server'
import { cn } from '@/lib/utils'

const COLOR: Record<Level, string> = { ok: 'bg-foreground', warn: 'bg-warn', bad: 'bg-bad' }

/** UsageBar 细进度条：<70% 前景色、70–90% 琥珀、≥90% 红；value 为 null 时只显示轨道 */
export function UsageBar({ value, className }: { value: number | null; className?: string }) {
  const v = value ?? 0
  return (
    <div
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={value == null ? undefined : Math.round(v)}
      className={cn('h-1 w-full overflow-hidden rounded-full bg-muted', className)}
    >
      {value != null && <div className={cn('h-full rounded-full transition-[width] duration-500', COLOR[usageLevel(v)])} style={{ width: `${v}%` }} />}
    </div>
  )
}
