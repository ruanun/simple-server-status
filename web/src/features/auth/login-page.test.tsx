import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { mockFetch } from '@/test/fetch'
import { renderWithProviders } from '@/test/render'

import { LoginPage } from './login-page'

describe('LoginPage', () => {
  it('失败时显示后端消息，成功后保存 token 并跳转后台', async () => {
    mockFetch({
      'GET /api/public/site': { site_title: '我的探针', show_price: false },
      'GET /api/auth/captcha': { mode: 'none' },
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

  it('图形验证码：随登录请求提交，失败后自动换一张', async () => {
    let n = 0
    const bodies: Record<string, unknown>[] = []
    mockFetch({
      'GET /api/public/site': { site_title: '我的探针', show_price: false },
      'GET /api/auth/captcha': () => {
        n += 1
        return { data: { mode: 'image', id: `c${n}`, image: `data:image/png;base64,${n}` } }
      },
      'POST /api/auth/login': (body: unknown) => {
        bodies.push(body as Record<string, unknown>)
        return (body as { captcha_code: string }).captcha_code === 'AB3C'
          ? { data: { token: 'tok', username: 'admin' } }
          : { status: 400, error: { code: 'captcha_invalid', message: '验证码错误或已过期' } }
      },
    })
    const user = userEvent.setup()
    renderWithProviders(<LoginPage />, { route: '/login', path: '/login' })

    expect(await screen.findByRole('img', { name: '验证码图片' })).toHaveAttribute('src', 'data:image/png;base64,1')
    await user.type(screen.getByLabelText('密码'), 'password123')
    await user.type(screen.getByLabelText('验证码'), 'XXXX')
    await user.click(screen.getByRole('button', { name: '登录' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('验证码错误或已过期')
    expect(bodies[0]).toMatchObject({ captcha_id: 'c1', captcha_code: 'XXXX' })
    await waitFor(() => expect(screen.getByRole('img', { name: '验证码图片' })).toHaveAttribute('src', 'data:image/png;base64,2'))
    expect(screen.getByLabelText('验证码')).toHaveValue('')

    await user.type(screen.getByLabelText('验证码'), 'AB3C')
    await user.click(screen.getByRole('button', { name: '登录' }))
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/admin'))
    expect(bodies[1]).toMatchObject({ captcha_id: 'c2', captcha_code: 'AB3C' })
  })

  it('Turnstile：完成验证前不能提交，token 随登录请求提交', async () => {
    let onToken: ((t: string) => void) | undefined
    vi.stubGlobal('turnstile', {
      render: (_el: HTMLElement, opts: { callback: (t: string) => void }) => {
        onToken = opts.callback
        return 'w1'
      },
      remove: vi.fn(),
    })
    let body: Record<string, unknown> | null = null
    mockFetch({
      'GET /api/public/site': { site_title: '我的探针', show_price: false },
      'GET /api/auth/captcha': { mode: 'turnstile', site_key: 'site' },
      'POST /api/auth/login': (b: unknown) => {
        body = b as Record<string, unknown>
        return { data: { token: 'tok', username: 'admin' } }
      },
    })
    const user = userEvent.setup()
    renderWithProviders(<LoginPage />, { route: '/login', path: '/login' })
    await user.type(screen.getByLabelText('密码'), 'password123')
    await waitFor(() => expect(onToken).toBeDefined())
    expect(screen.getByRole('button', { name: '登录' })).toBeDisabled()
    act(() => onToken?.('tok-1'))
    await user.click(screen.getByRole('button', { name: '登录' }))
    await waitFor(() => expect(body).toMatchObject({ turnstile_token: 'tok-1' }))
    vi.unstubAllGlobals()
  })

  it('验证码获取失败时提示并可重试，获取成功前不能提交', async () => {
    let fail = true
    mockFetch({
      'GET /api/public/site': { site_title: '我的探针', show_price: false },
      'GET /api/auth/captcha': () =>
        fail ? { status: 500, error: { code: 'internal', message: '服务器内部错误' } } : { data: { mode: 'image', id: 'c1', image: 'data:image/png;base64,1' } },
    })
    const user = userEvent.setup()
    renderWithProviders(<LoginPage />, { route: '/login', path: '/login' })
    expect(await screen.findByRole('alert')).toHaveTextContent('服务器内部错误')
    expect(screen.getByRole('button', { name: '登录' })).toBeDisabled()
    fail = false
    await user.click(screen.getByRole('button', { name: '重试' }))
    expect(await screen.findByRole('img', { name: '验证码图片' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '登录' })).toBeEnabled()
  })
})
