import { describe, expect, it } from 'vitest'

import { makeMetrics, makeServer } from '@/test/fixtures'

import { applyFilter, buildChips, sameFilter, sortBy } from './filters'

const servers = [
  makeServer({ id: 'a', name: 'alpha', group: 'HK', country: 'HK' }),
  makeServer({ id: 'b', name: 'beta', group: 'HK', country: 'JP', online: false }),
  makeServer({ id: 'c', name: 'gamma', group: '', country: '' }),
]

describe('buildChips', () => {
  it('统计分组、地区与离线数量（空值不计）', () => {
    const chips = buildChips(servers)
    expect(chips.groups).toEqual([{ value: 'HK', count: 2 }])
    expect(chips.countries).toEqual([
      { value: 'HK', count: 1 },
      { value: 'JP', count: 1 },
    ])
    expect(chips.offline).toBe(1)
  })
})

describe('applyFilter', () => {
  it('按分组、地区、离线与名称搜索过滤', () => {
    expect(applyFilter(servers, { kind: 'group', value: 'HK' }, '').map((s) => s.id)).toEqual(['a', 'b'])
    expect(applyFilter(servers, { kind: 'country', value: 'JP' }, '').map((s) => s.id)).toEqual(['b'])
    expect(applyFilter(servers, { kind: 'offline' }, '').map((s) => s.id)).toEqual(['b'])
    expect(applyFilter(servers, { kind: 'all' }, ' AL ').map((s) => s.id)).toEqual(['a'])
  })

  it('sameFilter 比较种类与值', () => {
    expect(sameFilter({ kind: 'group', value: 'HK' }, { kind: 'group', value: 'HK' })).toBe(true)
    expect(sameFilter({ kind: 'group', value: 'HK' }, { kind: 'country', value: 'HK' })).toBe(false)
    expect(sameFilter({ kind: 'all' }, { kind: 'all' })).toBe(true)
  })
})

describe('sortBy', () => {
  it('按指标排序，无数据的服务器排在升序末尾', () => {
    const list = [
      makeServer({ id: 'x', name: 'x', metrics: makeMetrics({ cpu: 50 }) }),
      makeServer({ id: 'y', name: 'y', metrics: null }),
      makeServer({ id: 'z', name: 'z', metrics: makeMetrics({ cpu: 90 }) }),
    ]
    expect(sortBy(list, 'cpu', true).map((s) => s.id)).toEqual(['z', 'x', 'y'])
    expect(sortBy(list, 'name', false).map((s) => s.id)).toEqual(['x', 'y', 'z'])
    expect(sortBy(list, 'expire', false).map((s) => s.id)).toEqual(['x', 'y', 'z'])
  })
})
