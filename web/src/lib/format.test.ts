import { describe, expect, it } from 'vitest'

import { daysUntil, formatAgo, formatBytes, formatChartLabel, formatChartTick, formatDuration, formatPercent, formatSpeed, percent } from './format'

describe('formatBytes', () => {
  it.each([
    [0, '0 B'],
    [-5, '0 B'],
    [512, '512 B'],
    [1024, '1.0 KB'],
    [1536, '1.5 KB'],
    [150 * 1024 ** 3, '150 GB'],
    [2.5 * 1024 ** 4, '2.5 TB'],
  ])('%d → %s', (n, want) => {
    expect(formatBytes(n)).toBe(want)
  })

  it('formatSpeed 附加 /s', () => {
    expect(formatSpeed(1024)).toBe('1.0 KB/s')
  })
})

describe('percent', () => {
  it('计算百分比并限制在 0–100', () => {
    expect(percent(50, 200)).toBe(25)
    expect(percent(1, 0)).toBe(0)
    expect(percent(300, 100)).toBe(100)
    expect(formatPercent(12.6)).toBe('13%')
  })
})

describe('formatDuration', () => {
  it('中文最多两级', () => {
    expect(formatDuration(23 * 86400 + 5 * 3600 + 10, 'zh-CN')).toBe('23 天 5 小时')
    expect(formatDuration(86400, 'zh-CN')).toBe('1 天')
    expect(formatDuration(2 * 3600 + 3 * 60, 'zh-CN')).toBe('2 小时 3 分')
    expect(formatDuration(59, 'zh-CN')).toBe('0 分钟')
  })
  it('英文缩写', () => {
    expect(formatDuration(23 * 86400 + 5 * 3600, 'en-US')).toBe('23d 5h')
    expect(formatDuration(3 * 60, 'en-US')).toBe('3m')
  })
})

describe('daysUntil', () => {
  it('向上取整，无到期时间返回 null', () => {
    expect(daysUntil(null, 0)).toBeNull()
    expect(daysUntil(2 * 86400, 0)).toBe(2)
    expect(daysUntil(100, 0)).toBe(1)
    expect(daysUntil(0, 86400)).toBe(-1)
  })
})

describe('formatAgo', () => {
  it('按量级输出', () => {
    expect(formatAgo(95, 100, 'zh-CN')).toBe('5 秒前')
    expect(formatAgo(0, 7200, 'en-US')).toBe('2h ago')
    expect(formatAgo(0, 3 * 86400, 'zh-CN')).toBe('3 天前')
  })
})

describe('图表时间格式', () => {
  // 用本地时间构造，结果与运行环境时区无关
  const ts = new Date(2026, 8, 25, 14, 5).getTime() / 1000

  it('按时间范围选择刻度格式', () => {
    expect(formatChartTick(ts, 'realtime', 'zh-CN')).toBe('14:05')
    expect(formatChartTick(ts, '1h', 'zh-CN')).toBe('14:05')
    expect(formatChartTick(ts, '6h', 'zh-CN')).toBe('14:05')
    expect(formatChartTick(ts, '24h', 'zh-CN')).toBe('9/25 14:05')
    expect(formatChartTick(ts, '7d', 'zh-CN')).toBe('9/25')
  })

  it('提示框标签始终带日期与时间，并按语言本地化', () => {
    expect(formatChartLabel(ts, 'zh-CN')).toBe('9/25 14:05')
    expect(formatChartLabel(ts, 'en-US')).toMatch(/^9\/25, 02:05\sPM$/)
  })
})
