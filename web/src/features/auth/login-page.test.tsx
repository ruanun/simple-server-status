import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { mockFetch } from '@/test/fetch'
import { renderWithProviders } from '@/test/render'

import { LoginPage } from './login-page'

describe('LoginPage', () => {
  it('失败时显示后端消息，成功后保存 token 并跳转后台', async () => {
    mockFetch({
      'GET /api/public/site': { site_title: '我的探针', show_price: false },
      'POST /api/auth/login': (body: unknown) =>
        (body as { password: string }).password === 'password123'
          ? { data: { token: 'tok', username: 'admin' } }
          : { status: 401, error: { code: 'invalid_credentials', message: '用户名或密码错误' } },
    })
    const user = userEvent.setup()
    renderWithProviders(<LoginPage />, { route: '/login', path: '/login' })

    await user.type(screen.getByLabelText('密码'), 'wrong')
    await user.click(screen.getByRole('button', { name: '登录' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('用户名或密码错误')

    await user.clear(screen.getByLabelText('密码'))
    await user.type(screen.getByLabelText('密码'), 'password123')
    await user.click(screen.getByRole('button', { name: '登录' }))
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/admin'))
    expect(localStorage.getItem('sss.token')).toBe('tok')
  })
})
