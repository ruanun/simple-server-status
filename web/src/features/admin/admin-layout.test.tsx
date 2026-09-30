import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { tokenStore } from '@/lib/api'
import { mockFetch } from '@/test/fetch'
import { renderWithProviders } from '@/test/render'

import { AdminLayout } from './admin-layout'

const SITE_ROUTE = { 'GET /api/public/site': { site_title: '我的探针', show_price: false } }

describe('AdminLayout', () => {
  it('me 接口返回 401 时清除本地登录态', async () => {
    tokenStore.set('tok')
    mockFetch({
      ...SITE_ROUTE,
      'GET /api/auth/me': () => ({ status: 401, error: { code: 'invalid_token', message: '登录已过期' } }),
    })
    renderWithProviders(<AdminLayout />, { route: '/admin/servers', path: '/admin/*' })

    await waitFor(() => expect(localStorage.getItem('sss.token')).toBeNull())
  })

  it('me 正常返回时点击退出登录清除 token；侧栏显示站点标题', async () => {
    tokenStore.set('tok')
    mockFetch({
      ...SITE_ROUTE,
      'GET /api/auth/me': { username: 'admin' },
    })
    const user = userEvent.setup()
    renderWithProviders(<AdminLayout />, { route: '/admin/servers', path: '/admin/*' })

    expect(await screen.findByText('我的探针')).toBeInTheDocument()

    await user.click(await screen.findByRole('button', { name: /退出登录/ }))
    await waitFor(() => expect(localStorage.getItem('sss.token')).toBeNull())
  })

  it('侧栏底部显示 Dashboard 版本，正式版本号前加 v', async () => {
    tokenStore.set('tok')
    mockFetch({ ...SITE_ROUTE, 'GET /api/auth/me': { username: 'admin', version: '2.0.0-beta.4' } })
    renderWithProviders(<AdminLayout />, { route: '/admin/servers', path: '/admin/*' })

    const version = await screen.findByText('v2.0.0-beta.4')
    expect(version.parentElement).toHaveTextContent('Simple Server Status')
  })

  it('开发版本原样显示', async () => {
    tokenStore.set('tok')
    mockFetch({ ...SITE_ROUTE, 'GET /api/auth/me': { username: 'admin', version: 'dev' } })
    renderWithProviders(<AdminLayout />, { route: '/admin/servers', path: '/admin/*' })

    expect(await screen.findByText('dev')).toBeInTheDocument()
  })

  it('退出登录后清除后台与登录态服务器列表缓存', async () => {
    tokenStore.set('tok')
    mockFetch({ ...SITE_ROUTE, 'GET /api/auth/me': { username: 'admin' } })
    const user = userEvent.setup()
    const { client } = renderWithProviders(<AdminLayout />, { route: '/admin/servers', path: '/admin/*' })
    client.setQueryData(['admin', 'servers'], [{ id: 'hidden' }])
    client.setQueryData(['servers', 'authed'], [{ id: 'hidden' }])
    client.setQueryData(['servers', 'public'], [])

    await user.click(await screen.findByRole('button', { name: /退出登录/ }))
    await waitFor(() => expect(localStorage.getItem('sss.token')).toBeNull())
    expect(client.getQueryData(['admin', 'servers'])).toBeUndefined()
    expect(client.getQueryData(['servers', 'authed'])).toBeUndefined()
    expect(client.getQueryData(['servers', 'public'])).toEqual([])
  })
})
