import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { UsageBar } from './usage-bar'

describe('UsageBar', () => {
  it('有值时输出 aria-valuenow', () => {
    render(<UsageBar value={42.4} />)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '42')
  })

  it('值为 null 时不输出 aria-valuenow', () => {
    render(<UsageBar value={null} />)
    expect(screen.getByRole('progressbar')).not.toHaveAttribute('aria-valuenow')
  })
})
