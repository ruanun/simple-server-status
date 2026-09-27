import { describe, expect, it, vi } from 'vitest'

import i18n from '@/i18n'
import { mockFetch } from '@/test/fetch'

import { api, ApiError, errorMessage, tokenStore } from './api'

describe('api', () => {
  it('解包 data 并携带 JSON 请求体与 token', async () => {
    tokenStore.set('tok')
    const fetchFn = mockFetch({ 'POST /api/x': (body: unknown) => ({ data: { echo: body } }) })
    const r = await api.post<{ echo: unknown }>('/api/x', { a: 1 })
    expect(r.echo).toEqual({ a: 1 })
    const init = fetchFn.mock.calls[0][1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers.Authorization).toBe('Bearer tok')
    expect(headers['Content-Type']).toBe('application/json')
  })

  it('错误信封转换为 ApiError', async () => {
    mockFetch({ 'GET /api/x': () => ({ status: 400, error: { code: 'invalid_input', message: '名称不能为空' } }) })
    const err = await api.get('/api/x').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).code).toBe('invalid_input')
    expect((err as ApiError).status).toBe(400)
    expect(errorMessage(err)).toBe('名称不能为空')
  })

  it('已知错误码按当前语言翻译', async () => {
    await i18n.changeLanguage('en-US')
    mockFetch({ 'POST /api/auth/login': () => ({ status: 401, error: { code: 'invalid_credentials', message: '用户名或密码错误' } }) })
    const err = await api.post('/api/auth/login', {}).catch((e: unknown) => e)
    expect((err as ApiError).code).toBe('invalid_credentials')
    expect((err as ApiError).status).toBe(401)
    expect(errorMessage(err)).toBe('Incorrect username or password')
  })

  it('未知错误码回退后端原文', async () => {
    await i18n.changeLanguage('en-US')
    mockFetch({ 'GET /api/x': () => ({ status: 400, error: { code: 'bad_range', message: '不支持的时间范围' } }) })
    const err = await api.get('/api/x').catch((e: unknown) => e)
    expect((err as ApiError).code).toBe('bad_range')
    expect(errorMessage(err)).toBe('不支持的时间范围')
  })

  it('非登录接口 401 时清除 token 并通知订阅者', async () => {
    tokenStore.set('tok')
    const listener = vi.fn()
    const unsubscribe = tokenStore.subscribe(listener)
    mockFetch({ 'GET /api/admin/servers': () => ({ status: 401, error: { code: 'unauthorized', message: '请先登录' } }) })
    await expect(api.get('/api/admin/servers')).rejects.toThrow('请先登录')
    expect(tokenStore.get()).toBeNull()
    expect(listener).toHaveBeenCalled()
    unsubscribe()
  })

  it('登录接口 401 不清除已有 token', async () => {
    tokenStore.set('tok')
    mockFetch({ 'POST /api/auth/login': () => ({ status: 401, error: { code: 'invalid_credentials', message: '用户名或密码错误' } }) })
    await expect(api.post('/api/auth/login', {})).rejects.toThrow('用户名或密码错误')
    expect(tokenStore.get()).toBe('tok')
  })

  it('网络错误转换为 network', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('failed') }))
    const err = await api.get('/api/x').catch((e: unknown) => e)
    expect((err as ApiError).code).toBe('network')
    expect((err as ApiError).message).toBe('网络错误，请检查连接')
  })

  it('中文界面优先使用后端原文，保留错误细节', async () => {
    await i18n.changeLanguage('zh-CN')
    mockFetch({ 'POST /api/admin/import': () => ({ status: 500, error: { code: 'internal', message: '导入服务器失败' } }) })
    const err = await api.post('/api/admin/import', {}).catch((e: unknown) => e)
    expect((err as ApiError).code).toBe('internal')
    expect(errorMessage(err)).toBe('导入服务器失败')
  })

  it('中文界面后端无原文时仍按错误码翻译', async () => {
    await i18n.changeLanguage('zh-CN')
    mockFetch({ 'GET /api/x': () => ({ status: 404, error: { code: 'not_found' } }) })
    const err = await api.get('/api/x').catch((e: unknown) => e)
    expect(errorMessage(err)).toBe('请求的内容不存在')
  })
})
