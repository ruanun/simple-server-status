import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { mockFetch } from '@/test/fetch'
import { makeServer } from '@/test/fixtures'
import { renderWithProviders } from '@/test/render'

import { StatusPage } from './status-page'

const SITE = { site_title: '我的探针', show_price: false }

describe('StatusPage', () => {
  it('显示汇总，支持离线筛选、搜索与切换列表视图', async () => {
    mockFetch({
      'GET /api/public/site': SITE,
      'GET /api/public/servers': [
        makeServer({ id: 'a', name: 'alpha', group: 'HK', country: 'HK' }),
        makeServer({ id: 'b', name: 'beta', group: 'JP', country: 'JP', online: false }),
      ],
    })
    const user = userEvent.setup()
    renderWithProviders(<StatusPage />)

    expect(await screen.findByText('alpha')).toBeInTheDocument()
    expect(screen.getByText('我的探针')).toBeInTheDocument()
    expect(screen.getByText('1 台离线')).toBeInTheDocument()

    expect(screen.getByRole('button', { name: /^全部/ })).toHaveAttribute('aria-pressed', 'true')
    await user.click(screen.getByRole('button', { name: /^离线/ }))
    expect(screen.getByRole('button', { name: /^离线/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /^全部/ })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.queryByText('alpha')).not.toBeInTheDocument()
    expect(screen.getByText('beta')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /^全部/ }))
    await user.type(screen.getByPlaceholderText('搜索名称'), 'alp')
    expect(screen.queryByText('beta')).not.toBeInTheDocument()

    await user.click(screen.getByRole('radio', { name: '列表视图' }))
    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(localStorage.getItem('sss.view')).toBe('list')

    // 行保留原生 row 语义，名称单元格内为真正的链接
    expect(screen.getAllByRole('row')[1]).not.toHaveAttribute('role')
    const link = screen.getByRole('link', { name: 'alpha' })
    link.focus()
    await user.keyboard('{Enter}')
    expect(screen.getByTestId('location')).toHaveTextContent('/server/a')
  })

  it('已有数据时后续请求失败仍显示列表', async () => {
    let calls = 0
    mockFetch({
      'GET /api/public/site': SITE,
      'GET /api/public/servers': () => {
        calls += 1
        return calls === 1 ? { data: [makeServer({ id: 'a', name: 'alpha' })] } : { status: 500, error: { code: 'internal', message: '服务器内部错误' } }
      },
    })
    const { client } = renderWithProviders(<StatusPage />)
    expect(await screen.findByText('alpha')).toBeInTheDocument()
    await client.refetchQueries({ queryKey: ['servers'] })
    await waitFor(() => expect(calls).toBe(2))
    await waitFor(() => expect(client.getQueryState(['servers', 'public'])?.status).toBe('error'))
    expect(screen.getByText('alpha')).toBeInTheDocument()
    expect(screen.queryByText('服务器内部错误')).not.toBeInTheDocument()
  })

  it('首次请求失败时显示错误', async () => {
    mockFetch({ 'GET /api/public/site': SITE, 'GET /api/public/servers': () => ({ status: 500, error: { code: 'internal' } }) })
    renderWithProviders(<StatusPage />)
    expect(await screen.findByText('服务器内部错误')).toBeInTheDocument()
  })

  it('没有服务器时显示提示', async () => {
    mockFetch({ 'GET /api/public/site': SITE, 'GET /api/public/servers': [] })
    renderWithProviders(<StatusPage />)
    expect(await screen.findByText('还没有服务器')).toBeInTheDocument()
  })
})
