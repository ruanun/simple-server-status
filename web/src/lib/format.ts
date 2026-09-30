import type { Range } from './types'

export type Lang = 'zh-CN' | 'en-US'

const UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

/** formatBytes 把字节数格式化为带单位的字符串（1024 进制） */
export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return '0 B'
  const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), UNITS.length - 1)
  const v = n / 1024 ** i
  const digits = i === 0 || v >= 100 ? 0 : 1
  return `${v.toFixed(digits)} ${UNITS[i]}`
}

export function formatSpeed(n: number): string {
  return `${formatBytes(n)}/s`
}

/** percent 计算百分比并限制在 0–100，total 非正数时返回 0 */
export function percent(used: number, total: number): number {
  if (!Number.isFinite(used) || !Number.isFinite(total) || total <= 0) return 0
  return Math.min(100, Math.max(0, (used / total) * 100))
}

export function formatPercent(p: number): string {
  return `${Math.round(p)}%`
}

/** formatDuration 把秒数格式化为最多两级的时长 */
export function formatDuration(seconds: number, lang: Lang): string {
  const s = Math.max(0, Math.floor(seconds))
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  const zh = lang === 'zh-CN'
  if (d > 0) return zh ? `${d} 天${h > 0 ? ` ${h} 小时` : ''}` : `${d}d${h > 0 ? ` ${h}h` : ''}`
  if (h > 0) return zh ? `${h} 小时${m > 0 ? ` ${m} 分` : ''}` : `${h}h${m > 0 ? ` ${m}m` : ''}`
  return zh ? `${m} 分钟` : `${m}m`
}

/** daysUntil 距到期的天数（向上取整，已过期为负数）；无到期时间返回 null */
export function daysUntil(expireAt: number | null, nowSec: number): number | null {
  if (expireAt == null) return null
  return Math.ceil((expireAt - nowSec) / 86400)
}

/** formatAgo 相对时间，如"12 秒前" */
export function formatAgo(ts: number, nowSec: number, lang: Lang): string {
  const diff = Math.max(0, Math.floor(nowSec - ts))
  const zh = lang === 'zh-CN'
  if (diff < 60) return zh ? `${diff} 秒前` : `${diff}s ago`
  if (diff < 3600) return zh ? `${Math.floor(diff / 60)} 分钟前` : `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return zh ? `${Math.floor(diff / 3600)} 小时前` : `${Math.floor(diff / 3600)}h ago`
  return zh ? `${Math.floor(diff / 86400)} 天前` : `${Math.floor(diff / 86400)}d ago`
}

export function formatDate(ts: number, lang: Lang): string {
  return new Date(ts * 1000).toLocaleDateString(lang)
}

/** formatDateTime 本地日期与时间，如 "2026/9/29 17:05:09" */
export function formatDateTime(ts: number, lang: Lang): string {
  return new Date(ts * 1000).toLocaleString(lang)
}

const DAY_TIME: Intl.DateTimeFormatOptions = { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }

/** formatChartTick 图表横轴刻度：1h/6h 及实时显示时:分，24h 显示月/日 时:分，7d 显示月/日 */
export function formatChartTick(ts: number, range: Range, lang: Lang): string {
  const d = new Date(ts * 1000)
  if (range === '24h') return d.toLocaleString(lang, DAY_TIME)
  if (range === '7d') return d.toLocaleDateString(lang, { month: 'numeric', day: 'numeric' })
  return d.toLocaleTimeString(lang, { hour: '2-digit', minute: '2-digit' })
}

/** formatChartLabel 图表提示框标签，始终带日期与时间 */
export function formatChartLabel(ts: number, lang: Lang): string {
  return new Date(ts * 1000).toLocaleString(lang, DAY_TIME)
}

/** formatAvailability 在线率：保留 1 位小数，100% 不带小数 */
export function formatAvailability(v: number): string {
  return v >= 100 ? '100%' : `${v.toFixed(1)}%`
}

/** formatMonthDay "2026-09-01" → "9/1" */
export function formatMonthDay(day: string): string {
  const [, m, d] = day.split('-')
  return `${Number(m)}/${Number(d)}`
}
