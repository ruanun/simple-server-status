import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'

import { SiteHeader } from '@/components/site-header'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { adminApi } from '@/lib/admin-api'
import { errorMessage, tokenStore } from '@/lib/api'
import { useToken } from '@/lib/auth'

export function LoginPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const token = useToken()
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  const from = (location.state as { from?: string } | null)?.from ?? '/admin'

  if (token) return <Navigate to={from} replace />

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    setError('')
    try {
      const r = await adminApi.login(username, password)
      tokenStore.set(r.token)
      navigate(from, { replace: true })
    } catch (err) {
      setError(errorMessage(err))
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
          {error && (
            <p role="alert" className="text-sm text-bad">
              {error}
            </p>
          )}
          <Button type="submit" className="w-full" disabled={pending}>
            {t('login.submit')}
          </Button>
        </form>
      </main>
    </>
  )
}
