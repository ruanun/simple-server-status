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
    const fetchFn = mockFetch({ 'GET /api/admin/servers': [] })
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
    mockFetch({ 'GET /api/admin/servers': [makeAdminServer({ hidden: true })] })
    renderWithProviders(<ServersPage />, { route: '/admin/servers', path: '/admin/servers' })
    const row = (await screen.findByText('hk-01')).closest('tr')
    if (!row) throw new Error('未找到行')
    expect(within(row).getByText('仅登录可见')).toHaveClass('sr-only')
  })
})
