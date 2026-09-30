import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Turnstile } from '@/components/turnstile'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { adminApi, adminKeys } from '@/lib/admin-api'
import { errorMessage } from '@/lib/api'
import type { CaptchaMode, CaptchaSettings, Settings } from '@/lib/types'
import { NO_AUTOFILL } from '@/lib/utils'

/** useDebounced 值停止变化 delay 毫秒后才更新，避免输入 Site Key 时每个字符都重新加载验证组件 */
function useDebounced<T>(value: T, delay: number): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setV(value), delay)
    return () => clearTimeout(id)
  }, [value, delay])
  return v
}

/**
 * CaptchaSettingsForm 登录验证码设置；保存时以缓存中最新的设置为基础只覆盖验证码字段。
 * 新启用 Turnstile 或修改其密钥时，需先在表单内完成一次验证，由后端用新密钥核验，避免配置错误后无法登录
 */
export function CaptchaSettingsForm({ initial }: { initial: Settings }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [c, setC] = useState<CaptchaSettings>(initial.captcha)
  const [saved, setSaved] = useState<CaptchaSettings>(initial.captcha)
  const [token, setToken] = useState<string | null>(null)
  const [widgetKey, setWidgetKey] = useState(0)
  const [pending, setPending] = useState(false)

  const siteKey = c.turnstile_site_key.trim()
  const secret = c.turnstile_secret.trim()
  const needVerify =
    c.mode === 'turnstile' && !(saved.mode === 'turnstile' && saved.turnstile_site_key === siteKey && saved.turnstile_secret === secret)
  const widgetSiteKey = useDebounced(siteKey, 600)

  const save = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    try {
      const latest = qc.getQueryData<Settings>(adminKeys.settings) ?? initial
      const res = await adminApi.saveSettings({ ...latest, captcha: c, ...(needVerify ? { turnstile_token: token ?? '' } : {}) })
      qc.setQueryData(adminKeys.settings, res)
      setSaved(res.captcha)
      setC(res.captcha)
      toast.success(t('settings.saved'))
    } catch (err) {
      toast.error(errorMessage(err))
      // token 已被后端核验消耗，需要重新验证
      if (needVerify) setWidgetKey((k) => k + 1)
    } finally {
      setPending(false)
    }
  }

  const modes: { value: CaptchaMode; label: string }[] = [
    { value: 'none', label: t('captcha.none') },
    { value: 'image', label: t('captcha.image') },
    { value: 'turnstile', label: 'Cloudflare Turnstile' },
  ]

  return (
    <form onSubmit={(e) => void save(e)} className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor="captcha_mode">{t('captcha.mode')}</Label>
        <Select value={c.mode} onValueChange={(v) => setC({ ...c, mode: v as CaptchaMode })}>
          <SelectTrigger id="captcha_mode" className="w-56">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {modes.map((m) => (
              <SelectItem key={m.value} value={m.value}>
                {m.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {c.mode === 'turnstile' && (
        <>
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="turnstile_site_key">Site Key</Label>
              <Input
                id="turnstile_site_key"
                autoComplete="off"
                {...NO_AUTOFILL}
                value={c.turnstile_site_key}
                onChange={(e) => setC({ ...c, turnstile_site_key: e.target.value })}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="turnstile_secret">Secret Key</Label>
              <Input
                id="turnstile_secret"
                type="password"
                // 密码框上 off 会被 Chrome 忽略，new-password 才能阻止填入已保存的密码
                autoComplete="new-password"
                {...NO_AUTOFILL}
                value={c.turnstile_secret}
                onChange={(e) => setC({ ...c, turnstile_secret: e.target.value })}
              />
            </div>
          </div>
          {needVerify && (
            <div className="space-y-2">
              <p className="text-xs text-muted-foreground">{t('captcha.verifyHint')}</p>
              {widgetSiteKey && widgetSiteKey === siteKey && <Turnstile key={`${widgetSiteKey}-${widgetKey}`} siteKey={widgetSiteKey} onToken={setToken} />}
            </div>
          )}
        </>
      )}
      <p className="text-xs text-muted-foreground">{t('captcha.rescueHint')}</p>
      <Button type="submit" disabled={pending || (needVerify && (!siteKey || !secret || !token))}>
        {t('captcha.save')}
      </Button>
    </form>
  )
}
