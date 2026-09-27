import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { Point } from '@/lib/types'
import { mockFetch } from '@/test/fetch'
import { makeServer } from '@/test/fixtures'
import { renderWithProviders } from '@/test/render'

import { DetailPage } from './detail-page'

vi.mock('./metric-charts', () => ({
  default: ({ points }: { points: Point[] }) => <div data-testid="charts">{points.length}</div>,
}))

const point: Point = { ts: 1, cpu: 1, mem: 1, disk: 1, net_in: 1, net_out: 1, load1: 1, tcp: 1 }
const SITE = { site_title: '我的探针', show_price: true }

describe('DetailPage', () => {
  it('显示信息网格、分区与历史图表，并可切换时间范围', async () => {
    mockFetch({
      'GET /api/public/site': SITE,
      'GET /api/public/servers': [makeServer({ price: 9.9, currency: '$', billing_cycle: 'yearly', expire_at: 1_900_000_000 })],
      'GET /api/public/servers/srv1/metrics?range=realtime': [point],
      'GET /api/public/servers/srv1/metrics?range=1h': [point, point],
    })
    const user = userEvent.setup()
    renderWithProviders(<DetailPage />, { route: '/server/srv1', path: '/server/:id' })

    expect(await screen.findByRole('heading', { name: 'hk-01' })).toBeInTheDocument()
    expect(screen.getByText('AMD EPYC 7B13 × 4')).toBeInTheDocument()
    expect(screen.getByText('负载 0.50 · 进程 120 · TCP 30')).toBeInTheDocument()
    expect(screen.getByText('每月 1 日重置')).toBeInTheDocument()
    expect(screen.getByText('$ 9.9 / 年付')).toBeInTheDocument()
    // 设置了价格时仍显示到期提醒与日期
    expect(screen.getByText(/天后到期/).parentElement).toHaveTextContent(/天后到期 · .+ 到期/)
    expect(screen.getByText('Agent 2.0.0')).toBeInTheDocument()
    expect(screen.getByText('/')).toBeInTheDocument()
    expect(await screen.findByTestId('charts')).toHaveTextContent('1')

    await user.click(screen.getByRole('tab', { name: '1 小时' }))
    await waitFor(() => expect(screen.getByTestId('charts')).toHaveTextContent('2'))
  })

  it('服务器不存在或已隐藏时提示', async () => {
    mockFetch({ 'GET /api/public/site': SITE, 'GET /api/public/servers': [] })
    renderWithProviders(<DetailPage />, { route: '/server/none', path: '/server/:id' })
    expect(await screen.findByText('服务器不存在或已隐藏')).toBeInTheDocument()
  })

  it('列表请求失败时显示错误信息而不是不存在', async () => {
    mockFetch({ 'GET /api/public/site': SITE, 'GET /api/public/servers': () => ({ status: 500, error: { code: 'internal' } }) })
    renderWithProviders(<DetailPage />, { route: '/server/srv1', path: '/server/:id' })
    expect(await screen.findByText('服务器内部错误')).toBeInTheDocument()
    expect(screen.queryByText('服务器不存在或已隐藏')).not.toBeInTheDocument()
  })

  it('历史数据请求失败时显示错误信息而不是暂无数据', async () => {
    mockFetch({
      'GET /api/public/site': SITE,
      'GET /api/public/servers': [makeServer()],
      'GET /api/public/servers/srv1/metrics?range=realtime': () => ({ status: 400, error: { code: 'bad_range', message: '不支持的时间范围' } }),
    })
    renderWithProviders(<DetailPage />, { route: '/server/srv1', path: '/server/:id' })
    expect(await screen.findByText('不支持的时间范围')).toBeInTheDocument()
    expect(screen.queryByText('暂无数据')).not.toBeInTheDocument()
  })
})
