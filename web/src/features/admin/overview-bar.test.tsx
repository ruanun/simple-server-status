import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { mockFetch } from '@/test/fetch'
import { renderWithProviders } from '@/test/render'

import { OverviewBar } from './overview-bar'

describe('OverviewBar', () => {
  it('费用与到期均无数据时分别显示「—」占位', async () => {
    mockFetch({ 'GET /api/admin/overview': { monthly_cost: [], expiring: [] } })
    renderWithProviders(<OverviewBar />)

    expect(await screen.findByText('月均费用：')).toBeInTheDocument()
    expect(screen.getByText('30 天内到期：')).toBeInTheDocument()
    expect(screen.getAllByText('—')).toHaveLength(2)
  })

  it('请求失败时显示错误信息', async () => {
    mockFetch({ 'GET /api/admin/overview': () => ({ status: 500, error: { code: 'internal', message: '读取总览失败' } }) })
    renderWithProviders(<OverviewBar />)

    const msg = await screen.findByText('读取总览失败')
    expect(msg).toHaveClass('text-bad')
    expect(screen.queryByText('月均费用：')).not.toBeInTheDocument()
  })
})
