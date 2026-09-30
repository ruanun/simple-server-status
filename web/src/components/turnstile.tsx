import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useTheme } from './theme-context'

interface TurnstileApi {
  render: (
    el: HTMLElement,
    opts: {
      sitekey: string
      theme: 'light' | 'dark'
      callback: (token: string) => void
      'expired-callback': () => void
      'error-callback': () => void
    },
  ) => string
  remove: (id: string) => void
}

declare global {
  interface Window {
    turnstile?: TurnstileApi
  }
}

const SCRIPT = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
let loading: Promise<void> | null = null

/** loadTurnstile 按需加载 Cloudflare Turnstile 脚本（全局只加载一次，失败后允许重试） */
function loadTurnstile(): Promise<void> {
  if (window.turnstile) return Promise.resolve()
  loading ??= new Promise<void>((resolve, reject) => {
    const s = document.createElement('script')
    s.src = SCRIPT
    s.async = true
    s.onload = () => resolve()
    s.onerror = () => {
      loading = null
      s.remove()
      reject(new Error('turnstile script load failed'))
    }
    document.head.appendChild(s)
  })
  return loading
}

/**
 * Turnstile Cloudflare 人机验证组件：通过后回调 token，过期或出错时回调 null。
 * token 只能使用一次，需要重新验证时由父组件更换 key 让组件重新挂载
 */
export function Turnstile({ siteKey, onToken }: { siteKey: string; onToken: (token: string | null) => void }) {
  const { t } = useTranslation()
  const { resolved } = useTheme()
  const ref = useRef<HTMLDivElement>(null)
  const cb = useRef(onToken)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    cb.current = onToken
  })

  useEffect(() => {
    let id: string | undefined
    let cancelled = false
    loadTurnstile()
      .then(() => {
        if (cancelled || !ref.current || !window.turnstile) return
        id = window.turnstile.render(ref.current, {
          sitekey: siteKey,
          theme: resolved,
          callback: (token) => cb.current(token),
          'expired-callback': () => cb.current(null),
          'error-callback': () => cb.current(null),
        })
      })
      .catch(() => {
        if (!cancelled) setFailed(true)
      })
    return () => {
      cancelled = true
      if (id) window.turnstile?.remove(id)
      cb.current(null)
    }
  }, [siteKey, resolved])

  if (failed) {
    return (
      <p role="alert" className="text-sm text-bad">
        {t('captcha.loadFailed')}
      </p>
    )
  }
  return <div ref={ref} className="min-h-[65px]" />
}
