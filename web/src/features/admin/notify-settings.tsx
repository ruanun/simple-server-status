import { useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { adminApi, adminKeys } from '@/lib/admin-api'
import { errorMessage } from '@/lib/api'
import type { NotifySettings, NotifyTestResult, Settings } from '@/lib/types'
import { NO_AUTOFILL } from '@/lib/utils'


type NumKey = 'offline_minutes' | 'load_cpu' | 'load_mem' | 'load_disk' | 'load_minutes' | 'expire_days' | 'traffic_percent'
type BoolKey = 'offline_enabled' | 'load_enabled' | 'expire_enabled' | 'traffic_enabled'

/** NotifySettingsForm 通知渠道、规则与测试；保存时以缓存中最新的设置为基础只覆盖通知字段 */
export function NotifySettingsForm({ initial }: { initial: Settings }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [n, setN] = useState<NotifySettings>(initial.notify)
  const [showToken, setShowToken] = useState(false)
  const [pending, setPending] = useState(false)
  const [result, setResult] = useState<NotifyTestResult | null>(null)
  const [testing, setTesting] = useState(false)

  const num = (key: NumKey, min: number, max: number) => (
    <Input
      id={key}
      type="number"
      min={min}
      max={max}
      className="w-24"
      value={n[key]}
      onChange={(e) => setN({ ...n, [key]: Number(e.target.value) })}
    />
  )
  const rule = (key: BoolKey, label: string, fields: ReactNode) => (
    <div className="space-y-2 rounded-md border p-3">
      <div className="flex items-center gap-2">
        <Switch id={key} checked={n[key]} onCheckedChange={(v) => setN({ ...n, [key]: v })} />
        <Label htmlFor={key}>{label}</Label>
      </div>
      {n[key] && <div className="flex flex-wrap items-center gap-3 text-sm">{fields}</div>}
    </div>
  )
  const labeled = (key: NumKey, label: string, min: number, max: number) => (
    <span className="flex items-center gap-2">
      <Label htmlFor={key} className="font-normal">
        {label}
      </Label>
      {num(key, min, max)}
    </span>
  )

  const save = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    try {
      const latest = qc.getQueryData<Settings>(adminKeys.settings) ?? initial
      const saved = await adminApi.saveSettings({ ...latest, notify: n })
      qc.setQueryData(adminKeys.settings, saved)
      toast.success(t('settings.saved'))
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setPending(false)
    }
  }

  const test = async () => {
    setResult(null)
    setTesting(true)
    try {
      setResult(await adminApi.testNotify(n))
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setTesting(false)
    }
  }

  const line = (name: string, v: string | null) =>
    v == null ? null : (
      <p key={name} className={v === 'ok' ? 'text-sm' : 'text-sm text-bad'}>
        {name}：{v === 'ok' ? t('notify.testOk') : v}
      </p>
    )

  return (
    <form onSubmit={(e) => void save(e)} className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor="webhook_url">{t('notify.webhook')}</Label>
        <Input
          id="webhook_url"
          name="notify_webhook_url"
          autoComplete="off"
          {...NO_AUTOFILL}
          placeholder="https://"
          value={n.webhook_url}
          onChange={(e) => setN({ ...n, webhook_url: e.target.value })}
        />
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="telegram_token">{t('notify.botToken')}</Label>
          <div className="flex gap-2">
            <Input
              id="telegram_token"
              name="notify_telegram_token"
              type={showToken ? 'text' : 'password'}
              // 密码框上 off 会被 Chrome 忽略，new-password 才能阻止填入已保存的密码
              autoComplete="new-password"
              {...NO_AUTOFILL}
              value={n.telegram_token}
              onChange={(e) => setN({ ...n, telegram_token: e.target.value })}
            />
            <Button type="button" variant="outline" onClick={() => setShowToken((v) => !v)}>
              {showToken ? t('notify.hide') : t('notify.show')}
            </Button>
          </div>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="telegram_chat_id">{t('notify.chatId')}</Label>
          <Input
            id="telegram_chat_id"
            name="notify_telegram_chat_id"
            autoComplete="off"
            {...NO_AUTOFILL}
            value={n.telegram_chat_id}
            onChange={(e) => setN({ ...n, telegram_chat_id: e.target.value })}
          />
        </div>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="notify_lang">{t('notify.lang')}</Label>
        <Select value={n.lang} onValueChange={(v) => setN({ ...n, lang: v as NotifySettings['lang'] })}>
          <SelectTrigger id="notify_lang" className="w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="zh-CN">中文</SelectItem>
            <SelectItem value="en-US">English</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {rule('offline_enabled', t('notify.offline'), labeled('offline_minutes', t('notify.offlineMinutes'), 1, 1440))}
      {rule(
        'load_enabled',
        t('notify.load'),
        <>
          {labeled('load_cpu', t('notify.loadCpu'), 1, 100)}
          {labeled('load_mem', t('notify.loadMem'), 1, 100)}
          {labeled('load_disk', t('notify.loadDisk'), 1, 100)}
          {labeled('load_minutes', t('notify.loadMinutes'), 1, 10)}
        </>,
      )}
      {rule('expire_enabled', t('notify.expire'), labeled('expire_days', t('notify.expireDays'), 1, 90))}
      {rule('traffic_enabled', t('notify.traffic'), labeled('traffic_percent', t('notify.trafficPercent'), 1, 100))}
      {result && (
        <div role="status" className="space-y-1">
          {line('Webhook', result.webhook)}
          {line('Telegram', result.telegram)}
          <Link to="/admin/events?tab=notify" className="text-sm underline underline-offset-2">
            {t('notify.viewLog')}
          </Link>
        </div>
      )}
      <div className="flex flex-wrap gap-2">
        <Button type="submit" disabled={pending}>
          {t('notify.save')}
        </Button>
        <Button type="button" variant="outline" disabled={testing} onClick={() => void test()}>
          {testing ? t('notify.testing') : t('notify.test')}
        </Button>
      </div>
    </form>
  )
}
