import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { makeMetrics, makeServer } from '@/test/fixtures'
import { renderWithProviders } from '@/test/render'

import { ServerCard } from './server-card'

const NOW = 1_700_000_000

describe('ServerCard', () => {
  it('显示名称、系统、2×2 指标与网速，并链接到详情页', () => {
    renderWithProviders(<ServerCard server={makeServer({ expire_at: NOW + 3 * 86400 })} now={NOW} />)
    const link = screen.getByRole('link')
    expect(link).toHaveAttribute('href', '/server/srv1')
    expect(screen.getByText('hk-01')).toBeInTheDocument()
    expect(screen.getByText('debian 12 · kvm · x86_64')).toBeInTheDocument()
    expect(screen.getByText('3 天后到期')).toHaveClass('text-warn')
    expect(screen.getByText('在线 3 天 2 小时')).toBeInTheDocument()
    expect(screen.getByText('12%')).toBeInTheDocument()
    expect(screen.getByText('50%')).toBeInTheDocument()
    expect(screen.getByText('400 GB / 1.0 TB')).toBeInTheDocument()
    expect(screen.getByText(/↓ 1\.0 KB\/s/)).toBeInTheDocument()
    expect(screen.getAllByRole('progressbar')).toHaveLength(4)
  })

  it('从未上报、无配额、长期的服务器显示占位而不报错', () => {
    const s = makeServer({
      online: false,
      metrics: null,
      static: null,
      last_seen: 0,
      traffic: { in: 0, out: 0, used: 0, limit: null, mode: 'sum', reset_day: 1, period: '2026-09-01' },
    })
    renderWithProviders(<ServerCard server={s} now={NOW} />)
    expect(screen.getByText('离线')).toBeInTheDocument()
    expect(screen.getByText('长期')).toBeInTheDocument()
    expect(screen.getByText('∞')).toBeInTheDocument()
    expect(screen.getAllByText('—')).toHaveLength(3)
    expect(screen.getByText('尚未上报')).toBeInTheDocument()
  })

  it('高负载时进度条变红', () => {
    renderWithProviders(<ServerCard server={makeServer({ metrics: makeMetrics({ cpu: 95 }) })} now={NOW} />)
    const cpuBar = screen.getAllByRole('progressbar')[0]
    expect(cpuBar.firstElementChild).toHaveClass('bg-bad')
  })
})
