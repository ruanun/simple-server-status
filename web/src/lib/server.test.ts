import { describe, expect, it } from 'vitest'

import { makeHello, makeMetrics, makeServer } from '@/test/fixtures'

import { cpuPct, diskPct, expiryLevel, memPct, osLabel, sortServers, trafficPct, usageLevel } from './server'

describe('usageLevel', () => {
  it('70/90 为阈值', () => {
    expect(usageLevel(69.9)).toBe('ok')
    expect(usageLevel(70)).toBe('warn')
    expect(usageLevel(89.9)).toBe('warn')
    expect(usageLevel(90)).toBe('bad')
  })
})

describe('指标百分比', () => {
  it('根据静态信息与上报计算', () => {
    const s = makeServer({ static: makeHello({ mem_total: 1000 }), metrics: makeMetrics({ cpu: 150, mem_used: 250, disk_used: 30, disk_total: 120 }) })
    expect(cpuPct(s)).toBe(100)
    expect(memPct(s)).toBe(25)
    expect(diskPct(s)).toBe(25)
  })
  it('从未上报时为 0', () => {
    const s = makeServer({ static: null, metrics: null })
    expect(cpuPct(s)).toBe(0)
    expect(memPct(s)).toBe(0)
    expect(diskPct(s)).toBe(0)
  })
})

describe('trafficPct', () => {
  it('无配额返回 null', () => {
    const base = makeServer().traffic
    expect(trafficPct({ ...base, limit: null })).toBeNull()
    expect(trafficPct({ ...base, limit: 0 })).toBeNull()
    expect(trafficPct({ ...base, used: 50, limit: 200 })).toBe(25)
  })
})

describe('expiryLevel', () => {
  it('7 天内琥珀、过期红色、长期正常', () => {
    expect(expiryLevel(null)).toBe('ok')
    expect(expiryLevel(30)).toBe('ok')
    expect(expiryLevel(7)).toBe('warn')
    expect(expiryLevel(0)).toBe('bad')
  })
})

describe('osLabel', () => {
  it('系统 · 虚拟化 · 架构', () => {
    expect(osLabel(makeServer())).toBe('debian 12 · kvm · x86_64')
    expect(osLabel(makeServer({ static: makeHello({ platform: '', virtualization: '' }) }))).toBe('linux · x86_64')
    expect(osLabel(makeServer({ static: null }))).toBe('')
  })
})

describe('sortServers', () => {
  it('按 sort 再按名称排序且不修改原数组', () => {
    const list = [makeServer({ id: 'b', name: 'b', sort: 1 }), makeServer({ id: 'c', name: 'c', sort: 0 }), makeServer({ id: 'a', name: 'a', sort: 0 })]
    expect(sortServers(list).map((s) => s.id)).toEqual(['a', 'c', 'b'])
    expect(list[0].id).toBe('b')
  })
})
