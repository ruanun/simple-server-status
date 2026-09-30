import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { mockFetch } from '@/test/fetch'
import { makeAdminServer } from '@/test/fixtures'
import { renderWithProviders } from '@/test/render'

import { InstallDialog } from './install-dialog'

describe('InstallDialog', () => {
  it('默认使用当前页面地址，修改面板地址后按新地址重新获取命令', async () => {
    const dashboards: string[] = []
    mockFetch({
      'GET /api/admin/servers/srv1/install': (_body: unknown, url: string) => {
        const d = new URL(url, 'http://x').searchParams.get('dashboard') ?? ''
        dashboards.push(d)
        return { data: { linux: `install --dashboard ${d}`, windows: 'iwr x' } }
      },
    })
    const user = userEvent.setup()
    renderWithProviders(<InstallDialog server={makeAdminServer()} onClose={() => {}} />)

    const dialog = await screen.findByRole('dialog', { name: '安装命令' })
    expect(await within(dialog).findByText(`install --dashboard ${window.location.origin}`)).toBeInTheDocument()

    const input = within(dialog).getByLabelText('面板地址')
    expect(input).toHaveValue(window.location.origin)
    await user.clear(input)
    await user.type(input, 'https://probe.example.com{Enter}')

    expect(await within(dialog).findByText('install --dashboard https://probe.example.com')).toBeInTheDocument()
    await waitFor(() => expect(dashboards).toEqual([window.location.origin, 'https://probe.example.com']))
  })

  it('升级模式显示升级标题与说明', async () => {
    mockFetch({ 'GET /api/admin/servers/srv1/install': { linux: 'curl x', windows: 'iwr x' } })
    renderWithProviders(<InstallDialog server={makeAdminServer({ id: 'srv1' })} mode="upgrade" onClose={() => {}} />)
    expect(await screen.findByRole('dialog', { name: '升级 Agent' })).toBeInTheDocument()
    expect(screen.getByText('在目标机器重新执行即可覆盖升级')).toBeInTheDocument()
  })
})
