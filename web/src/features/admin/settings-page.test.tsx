import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { describe, expect, it, vi } from 'vitest'

import type { NotifySettings, Settings } from '@/lib/types'
import { mockFetch } from '@/test/fetch'
import { renderWithProviders } from '@/test/render'

import { SettingsPage } from './settings-page'

// 备份面板导出/导入涉及敏感操作（导出含 Agent 密钥），toast 提示通过 mock sonner 断言文案，
// 避免在测试中额外渲染 <Toaster /> 引入动画与 Portal 带来的不确定性。
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn() } }))

const NOTIFY: NotifySettings = {
  webhook_url: '',
  telegram_token: '',
  telegram_chat_id: '',
  lang: 'zh-CN',
  offline_enabled: true,
  offline_minutes: 3,
  load_enabled: true,
  reboot_enabled: true,
  ip_change_enabled: true,
  expire_enabled: true,
  expire_days: 7,
  traffic_enabled: true,
  traffic_percent: 90,
}
const SETTINGS: Settings = {
  site_title: 'Simple Server Status',
  show_price: false,
  default_report_interval: 2,
  install_script_base: 'https://example.com/dl',
  announcement: '',
  notify: NOTIFY,
  events: { load_cpu: 90, load_mem: 90, load_disk: 90, load_minutes: 5 },
  captcha: { mode: 'none', turnstile_site_key: '', turnstile_secret: '' },
}

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

  it('导入成功后两个表单显示导入后的最新设置', async () => {
    let current: Settings = SETTINGS
    const imported: Settings = {
      ...SETTINGS,
      site_title: '导入后的标题',
      notify: { ...NOTIFY, webhook_url: 'https://imported.example.com/x' },
    }
    mockFetch({
      'GET /api/admin/settings': () => ({ data: current }),
      'POST /api/admin/import': () => {
        current = imported
        return { data: { servers: 1 } }
      },
    })
    const user = userEvent.setup()
    const { container } = renderWithProviders(<SettingsPage />, { route: '/admin/settings', path: '/admin/settings' })
    await screen.findByLabelText('站点标题')
    const input = container.querySelector<HTMLInputElement>('input[type="file"]')
    if (!input) throw new Error('未找到文件输入框')
    await user.upload(input, new File([JSON.stringify({ servers: [{ name: 's1' }] })], 'import.json', { type: 'application/json' }))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: '导入' }))
    await waitFor(() => expect(screen.getByLabelText('站点标题')).toHaveValue('导入后的标题'))
    expect(screen.getByLabelText('Webhook 地址')).toHaveValue('https://imported.example.com/x')
  })

  it('保存公告', async () => {
    let saved: unknown
    mockFetch({ 'GET /api/admin/settings': SETTINGS, 'PUT /api/admin/settings': (body: unknown) => ((saved = body), { data: body }) })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />)
    await user.type(await screen.findByLabelText('公告'), '周六维护')
    await user.click(screen.getAllByRole('button', { name: '保存' })[0])
    await waitFor(() => expect(saved).toMatchObject({ announcement: '周六维护', notify: NOTIFY }))
  })

  it('通知设置可保存并发送测试', async () => {
    let saved: unknown
    let tested: unknown
    mockFetch({
      'GET /api/admin/settings': SETTINGS,
      'PUT /api/admin/settings': (body: unknown) => ((saved = body), { data: body }),
      'POST /api/admin/notify/test': (body: unknown) => ((tested = body), { data: { webhook: 'ok', telegram: '对方返回 HTTP 401 Unauthorized' } }),
    })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />)
    await user.type(await screen.findByLabelText('Webhook 地址'), 'https://hook.example.com/x')
    await user.type(screen.getByLabelText('Bot Token'), '123:abc')
    await user.type(screen.getByLabelText('Chat ID'), '42')
    await user.clear(screen.getByLabelText('离线超过（分钟）'))
    await user.type(screen.getByLabelText('离线超过（分钟）'), '10')
    await user.click(screen.getByRole('button', { name: '发送测试' }))
    await waitFor(() => expect(tested).toMatchObject({ webhook_url: 'https://hook.example.com/x', telegram_token: '123:abc', offline_minutes: 10 }))
    expect(await screen.findByText('Webhook：发送成功')).toBeInTheDocument()
    expect(screen.getByText('Telegram：对方返回 HTTP 401 Unauthorized')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '查看通知记录' })).toHaveAttribute('href', '/admin/events?tab=notify')
    await user.click(screen.getByRole('button', { name: '保存通知设置' }))
    await waitFor(() => expect(saved).toMatchObject({ site_title: 'Simple Server Status', notify: { webhook_url: 'https://hook.example.com/x', offline_minutes: 10 } }))
  })

  it('检测规则可保存，只覆盖检测规则字段', async () => {
    let saved: unknown
    mockFetch({
      'GET /api/admin/settings': SETTINGS,
      'PUT /api/admin/settings': (body: unknown) => ((saved = body), { data: body }),
    })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />, { route: '/admin/settings?tab=rules', path: '/admin/settings' })
    const cpu = await screen.findByLabelText('CPU（%）')
    await user.clear(cpu)
    await user.type(cpu, '80')
    await user.click(screen.getByRole('button', { name: '保存检测规则' }))
    await waitFor(() => expect(saved).toMatchObject({ notify: NOTIFY, events: { load_cpu: 80, load_mem: 90, load_minutes: 5 } }))
  })

  it('两个表单互不吞掉未保存的编辑', async () => {
    let current: Settings = SETTINGS
    const saves: Settings[] = []
    mockFetch({
      'GET /api/admin/settings': () => ({ data: current }),
      'PUT /api/admin/settings': (body: unknown) => {
        current = body as Settings
        saves.push(current)
        return { data: body }
      },
    })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />)
    // 通知表单改了但不保存
    await user.type(await screen.findByLabelText('Webhook 地址'), 'https://hook.example.com/x')
    // 保存站点表单：只提交站点字段，通知字段以已保存的设置为准
    const title = screen.getByLabelText('站点标题')
    await user.clear(title)
    await user.type(title, '新标题')
    await user.click(screen.getByRole('button', { name: '保存' }))
    await waitFor(() => expect(saves).toHaveLength(1))
    expect(saves[0]).toMatchObject({ site_title: '新标题', notify: NOTIFY })
    await waitFor(() => expect(screen.getByRole('button', { name: '保存' })).toBeEnabled())
    // 通知表单中的输入仍然保留
    expect(screen.getByLabelText('Webhook 地址')).toHaveValue('https://hook.example.com/x')
    // 保存通知表单：提交体包含站点表单刚保存的新标题
    await user.click(screen.getByRole('button', { name: '保存通知设置' }))
    await waitFor(() => expect(saves).toHaveLength(2))
    expect(saves[1]).toMatchObject({ site_title: '新标题', notify: { webhook_url: 'https://hook.example.com/x' } })
    // 站点表单同样保留自己的编辑
    expect(screen.getByLabelText('站点标题')).toHaveValue('新标题')
  })

  it('发送测试进行中按钮禁用并显示进行中文案', async () => {
    const mocked = mockFetch({ 'GET /api/admin/settings': SETTINGS, 'POST /api/admin/notify/test': { webhook: 'ok', telegram: null } })
    let release = () => {}
    const gate = new Promise<void>((r) => (release = r))
    vi.stubGlobal('fetch', async (input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input) === '/api/admin/notify/test') await gate
      return mocked(input, init)
    })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />)
    await user.click(await screen.findByRole('button', { name: '发送测试' }))
    const busy = await screen.findByRole('button', { name: '发送中…' })
    expect(busy).toBeDisabled()
    release()
    expect(await screen.findByText('Webhook：发送成功')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '发送测试' })).toBeEnabled()
  })

  it('通知渠道输入框不被浏览器与密码管理器自动填充', async () => {
    mockFetch({ 'GET /api/admin/settings': SETTINGS })
    renderWithProviders(<SettingsPage />)
    const token = await screen.findByLabelText('Bot Token')
    expect(token).toHaveAttribute('autocomplete', 'new-password')
    for (const label of ['Webhook 地址', 'Chat ID']) {
      expect(screen.getByLabelText(label)).toHaveAttribute('autocomplete', 'off')
    }
    for (const el of [token, screen.getByLabelText('Webhook 地址'), screen.getByLabelText('Chat ID')]) {
      expect(el).toHaveAttribute('name')
      expect(el).toHaveAttribute('data-1p-ignore')
      expect(el).toHaveAttribute('data-lpignore', 'true')
      expect(el).toHaveAttribute('data-bwignore')
    }
  })

  it('修改密码表单带有隐藏的用户名，便于密码管理器更新对应账号', async () => {
    mockFetch({ 'GET /api/admin/settings': SETTINGS, 'GET /api/auth/me': { username: 'admin' } })
    renderWithProviders(<SettingsPage />)
    const oldPw = await screen.findByLabelText('原密码')
    const form = oldPw.closest('form') as HTMLFormElement
    await waitFor(() => expect(form.querySelector('input[autocomplete="username"]')).toHaveValue('admin'))
  })

  it('按标签页分区显示，默认站点，当前标签写入网址', async () => {
    mockFetch({ 'GET /api/admin/settings': SETTINGS })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />, { route: '/admin/settings', path: '/admin/settings' })
    expect(await screen.findByRole('tab', { name: '站点' })).toHaveAttribute('aria-selected', 'true')
    for (const name of ['检测规则', '通知', '账号与安全', '备份']) {
      expect(screen.getByRole('tab', { name })).toHaveAttribute('aria-selected', 'false')
    }
    await user.click(screen.getByRole('tab', { name: '备份' }))
    expect(screen.getByRole('tab', { name: '备份' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByTestId('location')).toHaveTextContent('/admin/settings?tab=backup')
  })

  it('网址中的标签直接打开对应分区，无效值回到站点', async () => {
    mockFetch({ 'GET /api/admin/settings': SETTINGS })
    const { unmount } = renderWithProviders(<SettingsPage />, { route: '/admin/settings?tab=notify', path: '/admin/settings' })
    expect(await screen.findByRole('tab', { name: '通知' })).toHaveAttribute('aria-selected', 'true')
    unmount()
    renderWithProviders(<SettingsPage />, { route: '/admin/settings?tab=xyz', path: '/admin/settings' })
    expect(await screen.findByRole('tab', { name: '站点' })).toHaveAttribute('aria-selected', 'true')
  })

  it('切换标签页不丢失其他分区未保存的编辑', async () => {
    mockFetch({ 'GET /api/admin/settings': SETTINGS })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />, { route: '/admin/settings', path: '/admin/settings' })
    await user.click(await screen.findByRole('tab', { name: '通知' }))
    await user.type(screen.getByLabelText('Chat ID'), '42')
    await user.click(screen.getByRole('tab', { name: '站点' }))
    await user.click(screen.getByRole('tab', { name: '通知' }))
    expect(screen.getByLabelText('Chat ID')).toHaveValue('42')
  })

  it('启用图形验证码并保存，不影响其他设置', async () => {
    let saved: Record<string, unknown> | null = null
    mockFetch({
      'GET /api/admin/settings': SETTINGS,
      'PUT /api/admin/settings': (body: unknown) => {
        saved = body as Record<string, unknown>
        return { data: body }
      },
    })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />, { route: '/admin/settings?tab=account', path: '/admin/settings' })
    await user.click(await screen.findByRole('combobox', { name: '验证方式' }))
    await user.click(await screen.findByRole('option', { name: '图形验证码' }))
    await user.click(screen.getByRole('button', { name: '保存验证码设置' }))
    await waitFor(() => expect(saved).toMatchObject({ site_title: 'Simple Server Status', captcha: { mode: 'image' } }))
    expect(saved).not.toHaveProperty('turnstile_token')
  })

  it('新启用 Turnstile 时需先完成验证，保存时附带 token', async () => {
    let onToken: ((t: string) => void) | undefined
    const render = vi.fn((_el: HTMLElement, opts: { sitekey: string; callback: (t: string) => void }) => {
      onToken = opts.callback
      return 'w1'
    })
    vi.stubGlobal('turnstile', { render, remove: vi.fn() })
    let saved: Record<string, unknown> | null = null
    mockFetch({
      'GET /api/admin/settings': SETTINGS,
      'PUT /api/admin/settings': (body: unknown) => {
        saved = body as Record<string, unknown>
        return { data: body }
      },
    })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />, { route: '/admin/settings?tab=account', path: '/admin/settings' })
    await user.click(await screen.findByRole('combobox', { name: '验证方式' }))
    await user.click(await screen.findByRole('option', { name: 'Cloudflare Turnstile' }))
    await user.type(screen.getByLabelText('Site Key'), 'site-key')
    await user.type(screen.getByLabelText('Secret Key'), 'secret-key')
    const btn = screen.getByRole('button', { name: '保存验证码设置' })
    expect(btn).toBeDisabled()
    await waitFor(() => expect(render).toHaveBeenCalledWith(expect.any(HTMLElement), expect.objectContaining({ sitekey: 'site-key' })))
    act(() => onToken?.('tok-1'))
    await waitFor(() => expect(btn).toBeEnabled())
    await user.click(btn)
    await waitFor(() =>
      expect(saved).toMatchObject({
        captcha: { mode: 'turnstile', turnstile_site_key: 'site-key', turnstile_secret: 'secret-key' },
        turnstile_token: 'tok-1',
      }),
    )
    vi.unstubAllGlobals()
  })
})
