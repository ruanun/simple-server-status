import { useQuery } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'

import { SiteFooter } from '@/components/site-footer'
import { SiteHeader } from '@/components/site-header'
import { Turnstile } from '@/components/turnstile'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { adminApi } from '@/lib/admin-api'
import { errorMessage, tokenStore } from '@/lib/api'
import { useToken } from '@/lib/auth'
import type { LoginInput } from '@/lib/types'

export function LoginPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const token = useToken()
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const [tsToken, setTsToken] = useState<string | null>(null)
  const [tsKey, setTsKey] = useState(0) // 递增以重新挂载 Turnstile，换取新 token
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  // 图形验证码一次性使用，不缓存；每次进入登录页都重新获取
  const captcha = useQuery({ queryKey: ['captcha'], queryFn: adminApi.captcha, gcTime: 0, staleTime: Infinity, refetchOnWindowFocus: false })
  const from = (location.state as { from?: string } | null)?.from ?? '/admin'

  if (token) return <Navigate to={from} replace />

  const c = captcha.data
  // 验证码无论对错都已作废：图形验证码换一张，Turnstile 重新验证；之前没拿到验证码时也重新获取
  const renewCaptcha = () => {
    if (c?.mode === 'turnstile') {
      setTsToken(null)
      setTsKey((k) => k + 1)
    } else if (c?.mode !== 'none') {
      setCode('')
      void captcha.refetch()
    }
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    setError('')
    const input: LoginInput = { username, password }
    if (c?.mode === 'image') Object.assign(input, { captcha_id: c.id, captcha_code: code })
    if (c?.mode === 'turnstile') input.turnstile_token = tsToken ?? ''
    try {
      const r = await adminApi.login(input)
      tokenStore.set(r.token)
      navigate(from, { replace: true })
    } catch (err) {
      setError(errorMessage(err))
      renewCaptcha()
    } finally {
      setPending(false)
    }
  }

  return (
    <>
      <SiteHeader />
      <main className="mx-auto flex max-w-sm flex-col px-4 py-16">
        <form onSubmit={(e) => void submit(e)} className="space-y-4 rounded-lg border bg-card p-6">
          <h1 className="text-lg font-semibold">{t('login.title')}</h1>
          <div className="space-y-1.5">
            <Label htmlFor="username">{t('login.username')}</Label>
            <Input id="username" autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="password">{t('login.password')}</Label>
            <Input id="password" type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required />
          </div>
          {c?.mode === 'image' && (
            <div className="space-y-1.5">
              <Label htmlFor="captcha_code">{t('captcha.code')}</Label>
              <div className="flex gap-2">
                <Input
                  id="captcha_code"
                  autoComplete="off"
                  autoCapitalize="characters"
                  spellCheck={false}
                  maxLength={8}
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  required
                />
                <button
                  type="button"
                  className="shrink-0 overflow-hidden rounded-md border"
                  title={t('captcha.refresh')}
                  onClick={() => void captcha.refetch()}
                >
                  <img src={c.image} alt={t('captcha.imageAlt')} className="h-9 w-auto" />
                </button>
              </div>
            </div>
          )}
          {c?.mode === 'turnstile' && <Turnstile key={tsKey} siteKey={c.site_key} onToken={setTsToken} />}
          {captcha.isError && (
            <div role="alert" className="flex items-center justify-between gap-2 text-sm text-bad">
              <span>{errorMessage(captcha.error)}</span>
              <Button type="button" variant="outline" size="sm" onClick={() => void captcha.refetch()}>
                {t('captcha.retry')}
              </Button>
            </div>
          )}
          {error && (
            <p role="alert" className="text-sm text-bad">
              {error}
            </p>
          )}
          <Button type="submit" className="w-full" disabled={pending || !c || (c.mode === 'turnstile' && !tsToken)}>
            {t('login.submit')}
          </Button>
        </form>
      </main>
      <SiteFooter />
    </>
  )
}
