import { vi } from 'vitest'

export interface MockResult {
  status?: number
  data?: unknown
  error?: { code: string; message: string }
}
export type MockHandler = (body: unknown, url: string) => MockResult

/** mockFetch 按 "METHOD /path" 匹配请求（先匹配完整路径含 query，再匹配去掉 query 的路径），返回后端信封格式的响应 */
export function mockFetch(routes: Record<string, unknown>) {
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? `${input.pathname}${input.search}` : input.url
    const method = init?.method ?? 'GET'
    const route = routes[`${method} ${url}`] ?? routes[`${method} ${url.split('?')[0]}`]
    if (route === undefined) {
      return new Response(JSON.stringify({ error: { code: 'not_found', message: `未模拟的请求 ${method} ${url}` } }), { status: 404 })
    }
    const body: unknown = typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
    const result: MockResult = typeof route === 'function' ? (route as MockHandler)(body, url) : { data: route }
    const status = result.status ?? 200
    const payload = status < 400 ? { data: result.data ?? null } : { error: result.error ?? { code: 'error', message: '错误' } }
    return new Response(JSON.stringify(payload), { status, headers: { 'Content-Type': 'application/json' } })
  })
  vi.stubGlobal('fetch', fn)
  return fn
}
