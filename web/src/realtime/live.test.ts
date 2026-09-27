import { describe, expect, it } from 'vitest'

import { makeServer } from '@/test/fixtures'

import { recordSpeed, resetSpeedHistory, useSpeedHistory } from './speed-history'
import { applyMessage, serversKey, wsUrl } from './use-live-servers'

describe('applyMessage', () => {
  it('snapshot 整体替换并排序', () => {
    const next = applyMessage([makeServer({ id: 'x' })], { type: 'snapshot', data: [makeServer({ id: 'b', name: 'b' }), makeServer({ id: 'a', name: 'a' })] })
    expect(next.map((s) => s.id)).toEqual(['a', 'b'])
  })

  it('delta 按 id 覆盖、保留其余', () => {
    const prev = [makeServer({ id: 'a', name: 'a', online: true }), makeServer({ id: 'b', name: 'b' })]
    const next = applyMessage(prev, { type: 'delta', data: [makeServer({ id: 'a', name: 'a', online: false })] })
    expect(next).toHaveLength(2)
    expect(next.find((s) => s.id === 'a')?.online).toBe(false)
  })
})

describe('serversKey / wsUrl', () => {
  it('登录与未登录使用不同缓存键', () => {
    expect(serversKey(true)).not.toEqual(serversKey(false))
  })

  it('按协议选择 ws/wss 并附带 token', () => {
    expect(wsUrl(null, { protocol: 'http:', host: 'a:8900' })).toBe('ws://a:8900/api/public/ws')
    expect(wsUrl('t k', { protocol: 'https:', host: 'a.com' })).toBe('wss://a.com/api/public/ws?token=t%20k')
  })
})

describe('recordSpeed', () => {
  it('汇总在线服务器网速，同一秒只保留最后一个样本', () => {
    resetSpeedHistory()
    recordSpeed([makeServer(), makeServer({ id: 'off', online: false })], 100)
    recordSpeed([makeServer()], 100)
    recordSpeed([makeServer()], 101)
    const samples = useSpeedHistory.getSnapshot()
    expect(samples).toHaveLength(2)
    expect(samples[0]).toEqual({ ts: 100, in: 1024, out: 2048 })
  })
})
