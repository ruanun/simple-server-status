import { percent } from './format'
import type { ServerView, Traffic } from './types'

export type Level = 'ok' | 'warn' | 'bad'

/** usageLevel 用量阈值：<70 正常，70–90 警告，≥90 危险 */
export function usageLevel(p: number): Level {
  if (p >= 90) return 'bad'
  if (p >= 70) return 'warn'
  return 'ok'
}

export function cpuPct(s: ServerView): number {
  return s.metrics ? Math.min(100, Math.max(0, s.metrics.cpu)) : 0
}

export function memPct(s: ServerView): number {
  return s.metrics && s.static ? percent(s.metrics.mem_used, s.static.mem_total) : 0
}

export function diskPct(s: ServerView): number {
  return s.metrics ? percent(s.metrics.disk_used, s.metrics.disk_total) : 0
}

/** trafficPct 流量配额使用率；未设置配额返回 null（界面显示 ∞） */
export function trafficPct(t: Traffic): number | null {
  return t.limit != null && t.limit > 0 ? percent(t.used, t.limit) : null
}

/** expiryLevel 到期提醒级别：≤7 天警告，已到期危险 */
export function expiryLevel(days: number | null): Level {
  if (days == null) return 'ok'
  if (days <= 0) return 'bad'
  if (days <= 7) return 'warn'
  return 'ok'
}

/** osLabel 形如"debian 12 · kvm · x86_64" */
export function osLabel(s: ServerView): string {
  const st = s.static
  if (!st) return ''
  const os = st.platform ? `${st.platform}${st.platform_version ? ` ${st.platform_version}` : ''}` : st.os
  return [os, st.virtualization, st.arch].filter(Boolean).join(' · ')
}

/** sortServers 按后台排序值、再按名称排序，返回新数组 */
export function sortServers(list: ServerView[]): ServerView[] {
  return [...list].sort((a, b) => a.sort - b.sort || a.name.localeCompare(b.name))
}
