import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { describe, expect, it, vi } from 'vitest'

import { tokenStore } from '@/lib/api'
import type { ServerView } from '@/lib/types'
import { FakeWebSocket } from '@/test/fake-ws'
import { makeServer } from '@/test/fixtures'

import { useLiveServers } from './use-live-servers'

/** stubServers 模拟 GET /api/public/servers：按请求是否携带 Authorization 返回不同列表 */
function stubServers(handler: (authed: boolean) => ServerView[]) {
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : String(input)
    if (!url.startsWith('/api/public/servers')) {
      return new Response(JSON.stringify({ error: { code: 'not_found', message: '未处理的请求' } }), { status: 404 })
    }
    const authed = Boolean((init?.headers as Record<string, string> | undefined)?.Authorization)
    return new Response(JSON.stringify({ data: handler(authed) }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  })
  vi.stubGlobal('fetch', fn)
  return fn
}

/** renderLive 用独立 QueryClient 包裹渲染 useLiveServers */
function renderLive() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  function wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>
  }
  return renderHook(() => useLiveServers(), { wrapper })
}

describe('useLiveServers', () => {
  it('挂载后请求服务器列表并建立 WebSocket 连接，收到推送后更新状态', async () => {
    stubServers(() => [makeServer({ id: 'a', name: 'a' })])
    const { result } = renderLive()

    expect(FakeWebSocket.instances).toHaveLength(1)
    expect(FakeWebSocket.latest().url).toBe(`ws://${window.location.host}/api/public/ws`)

    await waitFor(() => expect(result.current.servers.map((s) => s.id)).toEqual(['a']))

    act(() => FakeWebSocket.latest().open())
    expect(result.current.status).toBe('open')

    act(() => FakeWebSocket.latest().receive({ type: 'snapshot', data: [makeServer({ id: 'b', name: 'b' })] }))
    await waitFor(() => expect(result.current.servers.map((s) => s.id)).toEqual(['b']))

    act(() => FakeWebSocket.latest().receive({ type: 'delta', data: [makeServer({ id: 'b', name: 'b', online: false })] }))
    await waitFor(() => expect(result.current.servers.find((s) => s.id === 'b')?.online).toBe(false))
  })

  it('页面隐藏时关闭连接，恢复可见后重新建立连接', async () => {
    stubServers(() => [makeServer()])
    const { result } = renderLive()
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    expect(FakeWebSocket.instances).toHaveLength(1)
    const first = FakeWebSocket.latest()

    let hidden = true
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => hidden })
    act(() => document.dispatchEvent(new Event('visibilitychange')))
    expect(first.closed).toBe(true)
    expect(FakeWebSocket.instances).toHaveLength(1)

    hidden = false
    act(() => document.dispatchEvent(new Event('visibilitychange')))
    expect(FakeWebSocket.instances).toHaveLength(2)
    expect(FakeWebSocket.latest().closed).toBe(false)

    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
  })

  it('登录态变化后重新连接，并按公开/私有身份返回不同的服务器列表', async () => {
    const publicList = [makeServer({ id: 'a', name: 'a', hidden: false })]
    const authedList = [makeServer({ id: 'a', name: 'a', hidden: false }), makeServer({ id: 'hidden1', name: 'h', hidden: true })]
    const fetchFn = stubServers((authed) => (authed ? authedList : publicList))

    const { result } = renderLive()
    await waitFor(() => expect(result.current.servers.map((s) => s.id)).toEqual(['a']))
    expect(FakeWebSocket.instances).toHaveLength(1)
    const anonWs = FakeWebSocket.latest()
    expect(anonWs.url).toBe(`ws://${window.location.host}/api/public/ws`)

    act(() => tokenStore.set('tok'))
    expect(anonWs.closed).toBe(true)
    expect(FakeWebSocket.instances).toHaveLength(2)
    expect(FakeWebSocket.latest().url).toBe(`ws://${window.location.host}/api/public/ws?token=tok`)
    const authedWs = FakeWebSocket.latest()

    await waitFor(() => expect(result.current.servers.some((s) => s.id === 'hidden1')).toBe(true))
    const authedInit = fetchFn.mock.calls.at(-1)?.[1] as RequestInit
    expect((authedInit.headers as Record<string, string>).Authorization).toBe('Bearer tok')

    act(() => tokenStore.clear())
    expect(authedWs.closed).toBe(true)
    expect(FakeWebSocket.instances).toHaveLength(3)
    expect(FakeWebSocket.latest().url).toBe(`ws://${window.location.host}/api/public/ws`)

    await waitFor(() => expect(result.current.servers.some((s) => s.id === 'hidden1')).toBe(false))
  })
})
