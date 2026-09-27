import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Flag } from './flag'

describe('Flag', () => {
  it('有效代码渲染 4x3 国旗图片', () => {
    render(<Flag code="hk" />)
    const img = screen.getByRole('img', { name: 'HK' })
    expect(img.tagName).toBe('IMG')
    expect(img).toHaveAttribute('title', 'HK')
    expect(img.getAttribute('src')).toMatch(/4x3\/hk\.svg/)
  })

  it('无效代码或不存在的国旗不渲染', () => {
    const { container } = render(
      <>
        <Flag code="" />
        <Flag code="zz" />
        <Flag code="hkg" />
      </>,
    )
    expect(container).toBeEmptyDOMElement()
  })
})
