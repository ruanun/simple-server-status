import { act, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { ThemeProvider, useTheme } from './theme'

function Probe() {
  const { theme, resolved, setTheme } = useTheme()
  return (
    <button type="button" onClick={() => setTheme('dark')}>
      {theme}/{resolved}
    </button>
  )
}

describe('ThemeProvider', () => {
  it('默认跟随系统，切换后写入 localStorage 并设置 dark 类', () => {
    render(
      <ThemeProvider>
        <Probe />
      </ThemeProvider>,
    )
    expect(screen.getByRole('button')).toHaveTextContent('system/light')
    act(() => screen.getByRole('button').click())
    expect(screen.getByRole('button')).toHaveTextContent('dark/dark')
    expect(localStorage.getItem('sss.theme')).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })
})
