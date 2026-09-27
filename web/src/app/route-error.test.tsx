import { render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'

import i18n from '@/i18n'

import { RouteError } from './route-error'

function renderBroken() {
  const router = createMemoryRouter([
    {
      path: '/',
      errorElement: <RouteError />,
      loader: () => {
        throw new Error('Failed to fetch dynamically imported module')
      },
      element: <div />,
    },
  ])
  return render(<RouterProvider router={router} />)
}

describe('RouteError', () => {
  it('显示国际化的标题说明，并提供刷新与返回首页', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    renderBroken()
    expect(await screen.findByRole('heading', { name: '页面加载失败' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '刷新页面' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '返回首页' })).toHaveAttribute('href', '/')
    expect(screen.getByText('Failed to fetch dynamically imported module')).toBeInTheDocument()
  })

  it('英文界面显示英文文案', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    await i18n.changeLanguage('en-US')
    renderBroken()
    expect(await screen.findByRole('heading', { name: 'Failed to load the page' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reload' })).toBeInTheDocument()
  })
})
