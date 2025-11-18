/**
 * 数据格式化工具函数
 * 用于格式化服务器监控数据的显示
 * @author ruan
 */

import { getLocale } from '@/locales'

/**
 * 格式化运行时间
 * @param uptimeSeconds 运行时间（秒）
 * @returns 格式化后的字符串
 * @example
 * formatUptime(1562560) // 中文: "18天 6小时 32分"  英文: "18d 6h 32m"
 */
export function formatUptime(uptimeSeconds: number): string {
  if (uptimeSeconds === undefined || uptimeSeconds === null || uptimeSeconds === 0) {
    const locale = getLocale()
    return locale === 'zh-CN' ? '0天' : '0d'
  }

  const days = Math.floor(uptimeSeconds / (60 * 60 * 24))
  const hours = Math.floor((uptimeSeconds % (60 * 60 * 24)) / (60 * 60))
  const minutes = Math.floor((uptimeSeconds % (60 * 60)) / 60)

  const locale = getLocale()

  if (locale === 'zh-CN') {
    // 中文格式：18天 6小时 32分
    const parts: string[] = []
    if (days > 0) parts.push(`${days}天`)
    if (hours > 0) parts.push(`${hours}小时`)
    if (minutes > 0) parts.push(`${minutes}分`)
    if (parts.length === 0) parts.push('0天')
    return parts.join(' ')
  } else {
    // 英文格式：18d 6h 32m
    const parts: string[] = []
    if (days > 0) parts.push(`${days}d`)
    if (hours > 0) parts.push(`${hours}h`)
    if (minutes > 0) parts.push(`${minutes}m`)
    if (parts.length === 0) parts.push('0d')
    return parts.join(' ')
  }
}

/**
 * 格式化系统负载
 * @param load1 1分钟负载
 * @param load5 5分钟负载
 * @param load15 15分钟负载
 * @returns 格式化后的字符串 "1.2 / 1.5 / 1.9"
 */
export function formatLoad(load1: number, load5: number, load15: number): string {
  return `${load1.toFixed(1)} / ${load5.toFixed(1)} / ${load15.toFixed(1)}`
}

/**
 * 格式化百分比
 * @param percent 百分比数值
 * @returns 格式化后的整数百分比
 */
export function formatPercent(percent: number): number {
  return Math.round(percent)
}

/**
 * 格式化内存显示（带单位和百分比）
 * @param used 已用内存（字节）
 * @param total 总内存（字节）
 * @param readableBytes 字节转换函数
 * @returns 格式化后的字符串 "3.1GB / 5.0GB"
 */
export function formatMemory(
  used: number,
  total: number,
  readableBytes: (bytes: number) => string
): string {
  return `${readableBytes(used)} / ${readableBytes(total)}`
}

/**
 * 格式化时间间隔
 * 计算从指定时间到现在的时间差，并格式化为易读字符串
 *
 * @param startTime 开始时间
 * @param locale 语言环境（可选，默认从全局配置获取）
 * @returns 格式化后的字符串
 *
 * @example
 * ```typescript
 * const startTime = new Date('2024-01-01 10:00:00')
 * formatTimeInterval(startTime, 'zh-CN') // "2天 5小时" 或 "5小时 30分钟" 等
 * formatTimeInterval(startTime, 'en-US') // "2d 5h" 或 "5h 30m" 等
 * ```
 */
export function formatTimeInterval(startTime: Date | null, locale?: string): string {
  if (!startTime) return '-'

  const currentLocale = locale || getLocale()
  const now = new Date()
  const diffMs = now.getTime() - startTime.getTime()
  const diffSeconds = Math.floor(diffMs / 1000)
  const diffMinutes = Math.floor(diffSeconds / 60)
  const diffHours = Math.floor(diffMinutes / 60)
  const diffDays = Math.floor(diffHours / 24)

  if (currentLocale === 'zh-CN') {
    if (diffDays > 0) {
      const hours = diffHours % 24
      return `${diffDays}天 ${hours}小时`
    } else if (diffHours > 0) {
      const minutes = diffMinutes % 60
      return `${diffHours}小时 ${minutes}分钟`
    } else if (diffMinutes > 0) {
      return `${diffMinutes}分钟`
    } else {
      return `${diffSeconds}秒`
    }
  } else {
    if (diffDays > 0) {
      const hours = diffHours % 24
      return `${diffDays}d ${hours}h`
    } else if (diffHours > 0) {
      const minutes = diffMinutes % 60
      return `${diffHours}h ${minutes}m`
    } else if (diffMinutes > 0) {
      return `${diffMinutes}m`
    } else {
      return `${diffSeconds}s`
    }
  }
}

