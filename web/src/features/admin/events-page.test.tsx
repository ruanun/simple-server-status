import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { mockFetch } from '@/test/fetch'
import { makeAdminServer } from '@/test/fixtures'
import { renderWithProviders } from '@/test/render'

import { EventsPage } from './events-page'

const now = Math.floor(Date.now() / 1000)
const SERVERS = [makeAdminServer({ id: 'a', name: 'hk' }), makeAdminServer({ id: 'b', name: 'jp' })]

describe('EventsPage', () => {
  it('事件：显示类型、进行中与详情，可按服务器与类型筛选并写入网址', async () => {
    const fetchFn = mockFetch({
      'GET /api/admin/servers': SERVERS,
      'GET /api/admin/events?page=1&size=50': {
        items: [
          { id: 4, server_id: 'a', server_name: 'hk', kind: 'ip_change', start_at: now - 60, end_at: now - 60, duration: 0, detail: { ipv4: ['1.1.1.1', '2.2.2.2'] } },
          { id: 3, server_id: 'a', server_name: 'hk', kind: 'load_cpu', start_at: now - 300, end_at: null, duration: 300, detail: { threshold: 90, minutes: 5, peak: 97.25 } },
          { id: 1, server_id: 'b', server_name: 'jp', kind: 'offline', start_at: now - 3600, end_at: now - 3120, duration: 480, detail: {} },
        ],
        total: 3,
      },
      'GET /api/admin/events?server_id=b&page=1&size=50': {
        items: [{ id: 1, server_id: 'b', server_name: 'jp', kind: 'offline', start_at: now - 3600, end_at: now - 3120, duration: 480, detail: {} }],
        total: 1,
      },
      'GET /api/admin/events?server_id=b&kind=load_cpu%2Cload_mem%2Cload_disk&page=1&size=50': { items: [], total: 0 },
    })
    const user = userEvent.setup()
    renderWithProviders(<EventsPage />, { route: '/admin/events', path: '/admin/events' })
    expect(await screen.findByText('进行中')).toBeInTheDocument()
    expect(screen.getByText('CPU 高负载')).toBeInTheDocument()
    expect(screen.getByText('峰值 97.3%（阈值 90%，5 分钟均值）')).toBeInTheDocument()
    expect(screen.getByText('IPv4 1.1.1.1 → 2.2.2.2')).toBeInTheDocument()
    expect(screen.getByText('8 分钟')).toBeInTheDocument()
    const ipRow = screen.getByText('IP 变化').closest('tr') as HTMLElement
    expect(within(ipRow).getAllByText('—')).toHaveLength(2) // 瞬时事件没有结束时间与时长

    await user.click(screen.getByRole('combobox', { name: '服务器' }))
    await user.click(await screen.findByRole('option', { name: 'jp' }))
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('server=b'))
    await waitFor(() => expect(screen.queryByText('进行中')).not.toBeInTheDocument())
    await user.click(screen.getByRole('combobox', { name: '类型' }))
    await user.click(await screen.findByRole('option', { name: '高负载' }))
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('type=load'))
    expect(await screen.findByText('暂无记录')).toBeInTheDocument()
    expect(fetchFn).toHaveBeenCalledWith(expect.stringContaining('kind=load_cpu%2Cload_mem%2Cload_disk'), expect.anything())
  })

  it('通知记录：状态筛选、失败原因、点击展开正文', async () => {
    mockFetch({
      'GET /api/admin/servers': SERVERS,
      'GET /api/admin/notify-log?page=1&size=50': {
        items: [
          { id: 3, server_id: '', server_name: '', kind: 'test', channel: 'webhook', title: '[测试] Simple Server Status', message: '这是一条测试通知', status: 'sent', error: '', created_at: now, done_at: now },
          { id: 2, server_id: 'a', server_name: 'hk', kind: 'offline', channel: 'telegram', title: '[离线] hk', message: '服务器 hk 已离线 3 分钟', status: 'failed', error: '对方返回 HTTP 401', created_at: now - 60, done_at: now - 50 },
        ],
        total: 2,
      },
    })
    const user = userEvent.setup()
    renderWithProviders(<EventsPage />, { route: '/admin/events?tab=notify', path: '/admin/events' })
    const row = (await screen.findByText('对方返回 HTTP 401')).closest('tr') as HTMLElement
    expect(within(row).getByText('失败')).toBeInTheDocument()
    expect(screen.getByText('测试')).toBeInTheDocument()
    await user.click(within(row).getByRole('button', { name: '展开' }))
    expect(screen.getByText('服务器 hk 已离线 3 分钟')).toBeInTheDocument()
  })

  it('分页：有下一页时可翻页并写入网址', async () => {
    mockFetch({
      'GET /api/admin/servers': SERVERS,
      'GET /api/admin/events?page=1&size=50': {
        items: [{ id: 1, server_id: 'a', server_name: 'hk', kind: 'offline', start_at: now - 600, end_at: now - 300, duration: 300, detail: {} }],
        total: 120,
      },
      'GET /api/admin/events?page=2&size=50': { items: [], total: 120 },
    })
    const user = userEvent.setup()
    renderWithProviders(<EventsPage />, { route: '/admin/events', path: '/admin/events' })
    expect(await screen.findByText('第 1 / 3 页')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '下一页' }))
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('page=2'))
  })
})
