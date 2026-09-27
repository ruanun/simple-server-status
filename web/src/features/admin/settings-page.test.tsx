import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { describe, expect, it, vi } from 'vitest'

import type { Settings } from '@/lib/types'
import { mockFetch } from '@/test/fetch'
import { renderWithProviders } from '@/test/render'

import { SettingsPage } from './settings-page'

// 备份面板导出/导入涉及敏感操作（导出含 Agent 密钥），toast 提示通过 mock sonner 断言文案，
// 避免在测试中额外渲染 <Toaster /> 引入动画与 Portal 带来的不确定性。
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const SETTINGS: Settings = { site_title: 'Simple Server Status', show_price: false, default_report_interval: 2, install_script_base: 'https://example.com/dl' }

describe('SettingsPage', () => {
  it('修改站点标题并保存', async () => {
    let saved: unknown = null
    mockFetch({
      'GET /api/admin/settings': SETTINGS,
      'PUT /api/admin/settings': (body: unknown) => {
        saved = body
        return { data: body }
      },
    })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />, { route: '/admin/settings', path: '/admin/settings' })
    const input = await screen.findByLabelText('站点标题')
    await user.clear(input)
    await user.type(input, '新标题')
    await user.click(screen.getByRole('button', { name: '保存' }))
    await waitFor(() => expect(saved).toMatchObject({ site_title: '新标题', default_report_interval: 2 }))
  })

  it('两次新密码不一致时提示且不发送请求', async () => {
    const fetchFn = mockFetch({ 'GET /api/admin/settings': SETTINGS })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />, { route: '/admin/settings', path: '/admin/settings' })
    await user.type(screen.getByLabelText('原密码'), 'password123')
    await user.type(screen.getByLabelText('新密码'), 'newpass123')
    await user.type(screen.getByLabelText('确认新密码'), 'different1')
    await user.click(screen.getByRole('button', { name: '修改密码' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('两次输入的新密码不一致')
    expect(fetchFn.mock.calls.some(([url]) => String(url) === '/api/auth/password')).toBe(false)
  })

  it('修改密码成功后更新本地 token', async () => {
    mockFetch({ 'GET /api/admin/settings': SETTINGS, 'PUT /api/auth/password': { token: 'new-token' } })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />, { route: '/admin/settings', path: '/admin/settings' })
    await user.type(screen.getByLabelText('原密码'), 'password123')
    await user.type(screen.getByLabelText('新密码'), 'newpass123')
    await user.type(screen.getByLabelText('确认新密码'), 'newpass123')
    await user.click(screen.getByRole('button', { name: '修改密码' }))
    await waitFor(() => expect(localStorage.getItem('sss.token')).toBe('new-token'))
  })

  it('导出：确认弹窗后请求导出接口并触发下载', async () => {
    const fetchFn = mockFetch({ 'GET /api/admin/settings': SETTINGS, 'GET /api/admin/export': { servers: [{ id: '1', secret: 'sec-1' }] } })
    const createObjectURL = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:x')
    const revokeObjectURL = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    try {
      const user = userEvent.setup()
      renderWithProviders(<SettingsPage />, { route: '/admin/settings', path: '/admin/settings' })
      await screen.findByLabelText('站点标题')
      await user.click(screen.getByRole('button', { name: '导出 JSON' }))
      expect(await screen.findByText('导出文件包含密钥')).toBeInTheDocument()
      const dialog = screen.getByRole('alertdialog')
      await user.click(within(dialog).getByRole('button', { name: '导出 JSON' }))
      await waitFor(() => expect(click).toHaveBeenCalledTimes(1))
      expect(fetchFn.mock.calls.some(([url]) => String(url) === '/api/admin/export')).toBe(true)
      const anchor = click.mock.contexts[0] as HTMLAnchorElement
      expect(anchor.download).toMatch(/^sss-export-\d{4}-\d{2}-\d{2}\.json$/)
      expect(revokeObjectURL).toHaveBeenCalledWith('blob:x')
    } finally {
      createObjectURL.mockRestore()
      revokeObjectURL.mockRestore()
      click.mockRestore()
    }
  })

  it('导入无效 JSON 时提示错误且不发送请求', async () => {
    const fetchFn = mockFetch({ 'GET /api/admin/settings': SETTINGS })
    const user = userEvent.setup()
    const { container } = renderWithProviders(<SettingsPage />, { route: '/admin/settings', path: '/admin/settings' })
    await screen.findByLabelText('站点标题')
    const input = container.querySelector<HTMLInputElement>('input[type="file"]')
    if (!input) throw new Error('未找到文件输入框')
    const file = new File(['not json'], 'export.json', { type: 'application/json' })
    await user.upload(input, file)
    await waitFor(() => expect(vi.mocked(toast.error)).toHaveBeenCalledWith('文件不是有效的 JSON'))
    expect(fetchFn.mock.calls.some(([url]) => String(url) === '/api/admin/import')).toBe(false)
  })

  it('导入合法 JSON 后请求导入接口并提示已导入数量', async () => {
    let imported: unknown = null
    mockFetch({
      'GET /api/admin/settings': SETTINGS,
      'POST /api/admin/import': (body: unknown) => {
        imported = body
        return { data: { servers: 2 } }
      },
    })
    const user = userEvent.setup()
    const { container } = renderWithProviders(<SettingsPage />, { route: '/admin/settings', path: '/admin/settings' })
    await screen.findByLabelText('站点标题')
    const input = container.querySelector<HTMLInputElement>('input[type="file"]')
    if (!input) throw new Error('未找到文件输入框')
    const payload = { servers: [{ name: 's1' }, { name: 's2' }] }
    const file = new File([JSON.stringify(payload)], 'import.json', { type: 'application/json' })
    await user.upload(input, file)
    const dialog = await screen.findByRole('alertdialog')
    expect(dialog).toHaveTextContent('将导入 2 台服务器')
    expect(dialog).toHaveTextContent('站点设置会被文件中的设置覆盖')
    expect(imported).toBeNull()
    await user.click(within(dialog).getByRole('button', { name: '导入' }))
    await waitFor(() => expect(imported).toEqual(payload))
    await waitFor(() => expect(vi.mocked(toast.success)).toHaveBeenCalledWith('已导入 2 台服务器'))
  })

  it('导入确认弹窗中取消时不发送请求', async () => {
    const fetchFn = mockFetch({ 'GET /api/admin/settings': SETTINGS })
    const user = userEvent.setup()
    const { container } = renderWithProviders(<SettingsPage />, { route: '/admin/settings', path: '/admin/settings' })
    await screen.findByLabelText('站点标题')
    const input = container.querySelector<HTMLInputElement>('input[type="file"]')
    if (!input) throw new Error('未找到文件输入框')
    await user.upload(input, new File([JSON.stringify({ servers: [] })], 'import.json', { type: 'application/json' }))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: '取消' }))
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
    expect(fetchFn.mock.calls.some(([url]) => String(url) === '/api/admin/import')).toBe(false)
  })
})
