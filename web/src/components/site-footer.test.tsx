import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { renderWithProviders } from '@/test/render'

import { SiteFooter } from './site-footer'

describe('SiteFooter', () => {
  it('显示程序名、当前年份与作者，链接指向 GitHub 且在新窗口打开', () => {
    renderWithProviders(<SiteFooter />)

    const footer = screen.getByRole('contentinfo')
    expect(footer).toHaveTextContent(`©${new Date().getFullYear()} Created by Ruan`)
    const project = screen.getByRole('link', { name: 'Simple Server Status' })
    expect(project).toHaveAttribute('href', 'https://github.com/ruanun/simple-server-status')
    expect(project).toHaveAttribute('target', '_blank')
    expect(project).toHaveAttribute('rel', 'noopener noreferrer')
    expect(screen.getByRole('link', { name: 'Ruan' })).toHaveAttribute('href', 'https://github.com/ruanun')
  })

  it('不显示程序版本号', () => {
    renderWithProviders(<SiteFooter />)

    expect(screen.getByRole('contentinfo')).not.toHaveTextContent(/v?\d+\.\d+\.\d+/)
  })
})
