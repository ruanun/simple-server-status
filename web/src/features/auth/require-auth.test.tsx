import { act, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import { tokenStore } from '@/lib/api'

import { RequireAuth } from './require-auth'

function LoginProbe() {
  const location = useLocation()
  return <div>login from {(location.state as { from?: string } | null)?.from}</div>
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/login" element={<LoginProbe />} />
        <Route element={<RequireAuth />}>
          <Route path="/admin/servers" element={<div>admin page</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

describe('RequireAuth', () => {
  it('未登录时跳转登录页并记录来源', () => {
    renderAt('/admin/servers')
    expect(screen.getByText('login from /admin/servers')).toBeInTheDocument()
  })

  it('已登录时渲染子路由，token 被清除后立即跳转', () => {
    tokenStore.set('tok')
    renderAt('/admin/servers')
    expect(screen.getByText('admin page')).toBeInTheDocument()
    act(() => tokenStore.clear())
    expect(screen.getByText('login from /admin/servers')).toBeInTheDocument()
  })
})
