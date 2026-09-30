import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Announcement } from './announcement'

describe('Announcement', () => {
  it('链接不吞掉紧随其后的中文标点', () => {
    render(<Announcement text="详见 https://a.com/x，谢谢" />)
    expect(screen.getByRole('link')).toHaveAttribute('href', 'https://a.com/x')
    expect(screen.getByRole('note')).toHaveTextContent('详见 https://a.com/x，谢谢')
  })

  it('句末的英文标点不计入链接', () => {
    render(<Announcement text="见 https://a.com." />)
    expect(screen.getByRole('link')).toHaveAttribute('href', 'https://a.com')
    expect(screen.getByRole('link')).toHaveTextContent(/^https:\/\/a\.com$/)
    expect(screen.getByRole('note')).toHaveTextContent('见 https://a.com.')
  })

  it('链接中合法的查询参数与括号后的标点', () => {
    render(<Announcement text="(见 https://a.com/p?q=1&b=2)!" />)
    expect(screen.getByRole('link')).toHaveAttribute('href', 'https://a.com/p?q=1&b=2')
    expect(screen.getByRole('note')).toHaveTextContent('(见 https://a.com/p?q=1&b=2)!')
  })
})
