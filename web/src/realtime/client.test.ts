import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { makeServer } from '@/test/fixtures'
import { FakeWebSocket } from '@/test/fake-ws'

import { backoff, parseMessage, RealtimeClient, type RealtimeStatus } from './client'

function setup() {
  const statuses: RealtimeStatus[] = []
  const messages: unknown[] = []
  const poll = vi.fn()
  const client = new RealtimeClient({
    url: () => 'ws://test/api/public/ws',
    onMessage: (m) => messages.push(m),
    onStatus: (s) => statuses.push(s),
    poll,
    random: () => 0.5,
    createSocket: (u) => new FakeWebSocket(u) as unknown as WebSocket,
  })
  return { client, statuses, messages, poll }
}

describe('backoff', () => {
  it('1s 起翻倍、30s 封顶、带抖动', () => {
    expect(backoff(0, () => 0.5)).toBe(1000)
    expect(backoff(3, () => 0.5)).toBe(8000)
    expect(backoff(5, () => 0.5)).toBe(30000)
    expect(backoff(20, () => 0.5)).toBe(30000)
    expect(backoff(0, () => 0)).toBe(800)
  })
})

describe('parseMessage', () => {
  it('只接受 snapshot/delta 且 data 为数组', () => {
    expect(parseMessage(JSON.stringify({ type: 'delta', data: [] }))).toEqual({ type: 'delta', data: [] })
    expect(parseMessage(JSON.stringify({ type: 'other', data: [] }))).toBeNull()
    expect(parseMessage('not json')).toBeNull()
    expect(parseMessage(42)).toBeNull()
  })
})

describe('RealtimeClient', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('连接成功后分发消息，忽略非法消息', () => {
    const { client, statuses, messages } = setup()
    client.start()
    expect(FakeWebSocket.latest().url).toBe('ws://test/api/public/ws')
    FakeWebSocket.latest().open()
    FakeWebSocket.latest().receive({ type: 'snapshot', data: [makeServer()] })
    FakeWebSocket.latest().receive('garbage')
    expect(statuses).toEqual(['connecting', 'open'])
    expect(messages).toHaveLength(1)
    client.stop()
  })

  it('断开后立即轮询、按退避重连，恢复后停止轮询', () => {
    const { client, statuses, poll } = setup()
    client.start()
    FakeWebSocket.latest().open()
    FakeWebSocket.latest().drop()
    expect(statuses.at(-1)).toBe('polling')
    expect(poll).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(999)
    expect(FakeWebSocket.instances).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(FakeWebSocket.instances).toHaveLength(2)
    vi.advanceTimersByTime(5000)
    expect(poll).toHaveBeenCalledTimes(2)
    FakeWebSocket.latest().open()
    expect(statuses.at(-1)).toBe('open')
    vi.advanceTimersByTime(20000)
    expect(poll).toHaveBeenCalledTimes(2)
    client.stop()
  })

  it('stop 后关闭连接且不再重连', () => {
    const { client } = setup()
    client.start()
    const ws = FakeWebSocket.latest()
    client.stop()
    expect(ws.closed).toBe(true)
    ws.drop()
    vi.advanceTimersByTime(60000)
    expect(FakeWebSocket.instances).toHaveLength(1)
  })
})
