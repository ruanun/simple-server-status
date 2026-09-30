import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import type { AdminServer } from '@/lib/types'
import { mockFetch } from '@/test/fetch'
import { makeAdminServer } from '@/test/fixtures'
import { renderWithProviders } from '@/test/render'

import { ServersPage } from './servers-page'

describe('ServersPage', () => {
  it('列出服务器，新建成功后弹出安装命令', async () => {
    let list: AdminServer[] = [makeAdminServer()]
    const created = makeAdminServer({ id: 'new1', name: 'n1', online: false })
    mockFetch({
      'GET /api/admin/servers': () => ({ data: list }),
      'POST /api/admin/servers': (body: unknown) => {
        list = [...list, { ...created, ...(body as object) }]
        return { data: created }
      },
      'GET /api/admin/servers/new1/install': { linux: 'curl -fsSL x | sudo bash -s -- --dashboard http://localhost:3000', windows: 'iwr x' },
      'GET /api/admin/overview': { monthly_cost: [], expiring: [] },
    })
    const user = userEvent.setup()
    renderWithProviders(<ServersPage />, { route: '/admin/servers', path: '/admin/servers' })

    expect(await screen.findByText('hk-01')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '新建服务器' }))
    expect(screen.getByLabelText('名称')).not.toHaveAttribute('aria-invalid')
    await user.type(screen.getByLabelText('名称'), 'n1')
    await user.click(screen.getByRole('button', { name: '保存' }))

    const dialog = await screen.findByRole('dialog', { name: '安装命令' })
    expect(await within(dialog).findByText(/--dashboard/)).toBeInTheDocument()
  })

  it('名称为空时显示校验错误且不提交', async () => {
    const fetchFn = mockFetch({ 'GET /api/admin/servers': [], 'GET /api/admin/overview': { monthly_cost: [], expiring: [] } })
    const user = userEvent.setup()
    renderWithProviders(<ServersPage />, { route: '/admin/servers', path: '/admin/servers' })
    expect(await screen.findByText('还没有服务器，点击右上角新建')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '新建服务器' }))
    // 对话框关闭按钮的读屏文本走词典
    expect(screen.getByRole('button', { name: '关闭' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '保存' }))
    const error = await screen.findByText('请输入名称')
    const name = screen.getByLabelText('名称')
    expect(name).toHaveAttribute('aria-invalid', 'true')
    expect(name).toHaveAttribute('aria-describedby', error.id)
    expect(screen.getByLabelText('国家代码')).toHaveAccessibleDescription('两位字母，如 HK；留空则使用 Agent 探测结果')
    expect(fetchFn.mock.calls.some(([, init]) => (init as RequestInit | undefined)?.method === 'POST')).toBe(false)
  })

  it('隐藏服务器的图标带可读标签', async () => {
    mockFetch({ 'GET /api/admin/servers': [makeAdminServer({ hidden: true })], 'GET /api/admin/overview': { monthly_cost: [], expiring: [] } })
    renderWithProviders(<ServersPage />, { route: '/admin/servers', path: '/admin/servers' })
    const row = (await screen.findByText('hk-01')).closest('tr')
    if (!row) throw new Error('未找到行')
    expect(within(row).getByText('仅登录可见')).toHaveClass('sr-only')
  })

  it('显示最后上报时间，从未上报时提示', async () => {
    const now = Math.floor(Date.now() / 1000)
    mockFetch({
      'GET /api/admin/servers': [
        makeAdminServer({ id: 'a', name: 'a1', last_seen: now - 5 }),
        makeAdminServer({ id: 'b', name: 'b1', last_seen: 0, online: false }),
      ],
      'GET /api/admin/overview': { monthly_cost: [], expiring: [] },
    })
    renderWithProviders(<ServersPage />, { route: '/admin/servers', path: '/admin/servers' })

    expect(await screen.findByText('a1')).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: '最后上报' })).toBeInTheDocument()
    const recent = screen.getByText(/^[56] 秒前$/)
    expect(recent).toHaveAttribute('title', expect.stringMatching(/\d{4}\/\d{1,2}\/\d{1,2} \d{2}:\d{2}:\d{2}/))
    expect(screen.getByText('尚未上报')).toBeInTheDocument()
  })

  it('显示总览、Agent 版本、在线率、备注与静音，旧版本可升级', async () => {
    const now = Math.floor(Date.now() / 1000)
    mockFetch({
      'GET /api/admin/servers': [
        makeAdminServer({ id: 'a', name: 'a1', agent_version: '2.0.0-beta.1', outdated: true, note: '搬瓦工 CN2', notify_muted: true, uptime_24h: 97 }),
        makeAdminServer({ id: 'b', name: 'b1' }),
      ],
      'GET /api/admin/overview': {
        monthly_cost: [{ currency: '$', amount: 12.5, servers: 2 }, { currency: '¥', amount: 30, servers: 1 }],
        expiring: [{ id: 'a', name: 'a1', expire_at: now + 5 * 86400, days: 5 }],
      },
      'GET /api/admin/servers/a/install': { linux: 'curl x', windows: 'iwr x' },
    })
    const user = userEvent.setup()
    renderWithProviders(<ServersPage />, { route: '/admin/servers', path: '/admin/servers' })

    expect(await screen.findByText('$ 12.5 · ¥ 30')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '30 天内到期 1 台' }))
    expect(screen.getByText((_, el) => el?.tagName === 'LI' && el.textContent === 'a1 · 5 天后到期')).toBeInTheDocument()

    const row = screen.getByText('a1').closest('tr') as HTMLElement
    expect(within(row).getByText('2.0.0-beta.1')).toHaveClass('text-warn')
    expect(within(row).getByText('97.0%')).toBeInTheDocument()
    expect(within(row).getByTitle('搬瓦工 CN2')).toBeInTheDocument()
    expect(within(row).getByText('备注：搬瓦工 CN2')).toHaveClass('sr-only')
    expect(within(row).getByTitle('不发送通知')).toBeInTheDocument()
    expect(within(row).getByText('不发送通知')).toHaveClass('sr-only')

    await user.click(within(row).getByRole('button', { name: '操作' }))
    await user.click(await screen.findByRole('menuitem', { name: '升级 Agent' }))
    expect(await screen.findByRole('dialog', { name: '升级 Agent' })).toBeInTheDocument()

    const other = screen.getByText('b1').closest('tr') as HTMLElement
    await user.keyboard('{Escape}')
    await user.click(within(other).getByRole('button', { name: '操作' }))
    expect(screen.queryByRole('menuitem', { name: '升级 Agent' })).not.toBeInTheDocument()
  })
})
