import { describe, expect, it } from 'vitest'

import i18n from '@/i18n'

describe('i18n 复数', () => {
  it('英文按数量选择单复数', () => {
    const t = i18n.getFixedT('en-US')
    expect(t('metric.cores', { count: 1 })).toBe('1 core')
    expect(t('metric.cores', { count: 2 })).toBe('2 cores')
    expect(t('settings.importDone', { count: 1 })).toBe('Imported 1 server')
    expect(t('settings.importDone', { count: 3 })).toBe('Imported 3 servers')
    expect(t('summary.offline', { count: 1 })).toBe('1 offline')
  })

  it('中文仍使用单个键', () => {
    const t = i18n.getFixedT('zh-CN')
    expect(t('metric.cores', { count: 1 })).toBe('1 核')
    expect(t('settings.importDone', { count: 2 })).toBe('已导入 2 台服务器')
  })
})
