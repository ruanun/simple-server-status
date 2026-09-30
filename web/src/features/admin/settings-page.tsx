import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useRef, useState, type ChangeEvent, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { adminApi, adminKeys } from '@/lib/admin-api'
import { errorMessage, tokenStore } from '@/lib/api'
import { downloadJson } from '@/lib/download'
import type { Settings } from '@/lib/types'

import { NotifySettingsForm } from './notify-settings'

function Card({ title, desc, children }: { title: string; desc?: string; children: ReactNode }) {
  return (
    <section className="space-y-4 rounded-lg border bg-card p-5">
      <div>
        <h2 className="text-sm font-medium">{title}</h2>
        {desc && <p className="mt-1 text-xs text-muted-foreground">{desc}</p>}
      </div>
      {children}
    </section>
  )
}

/** SiteSettingsForm 站点设置；保存时以缓存中最新的设置为基础只覆盖站点字段，避免吞掉通知表单已保存的内容 */
function SiteSettingsForm({ initial }: { initial: Settings }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [s, setS] = useState(initial)
  const [pending, setPending] = useState(false)

  const save = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    try {
      const latest = qc.getQueryData<Settings>(adminKeys.settings) ?? initial
      const saved = await adminApi.saveSettings({
        ...latest,
        site_title: s.site_title,
        show_price: s.show_price,
        default_report_interval: Number(s.default_report_interval),
        install_script_base: s.install_script_base,
        announcement: s.announcement,
      })
      // 直接更新缓存而不让表单重新挂载，另一个表单中未保存的编辑得以保留
      qc.setQueryData(adminKeys.settings, saved)
      await qc.invalidateQueries({ queryKey: ['site'] })
      toast.success(t('settings.saved'))
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setPending(false)
    }
  }

  return (
    <form onSubmit={(e) => void save(e)} className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor="site_title">{t('settings.siteTitle')}</Label>
        <Input id="site_title" value={s.site_title} onChange={(e) => setS({ ...s, site_title: e.target.value })} />
      </div>
      <div className="flex items-center gap-2">
        <Switch id="show_price" checked={s.show_price} onCheckedChange={(v) => setS({ ...s, show_price: v })} />
        <Label htmlFor="show_price">{t('settings.showPrice')}</Label>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="default_report_interval">{t('settings.defaultInterval')}</Label>
        <Input
          id="default_report_interval"
          type="number"
          min={1}
          max={60}
          value={s.default_report_interval}
          onChange={(e) => setS({ ...s, default_report_interval: Number(e.target.value) })}
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="install_script_base">{t('settings.scriptBase')}</Label>
        <Input id="install_script_base" value={s.install_script_base} onChange={(e) => setS({ ...s, install_script_base: e.target.value })} />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="announcement">{t('settings.announcement')}</Label>
        <Textarea id="announcement" rows={3} maxLength={1000} value={s.announcement} onChange={(e) => setS({ ...s, announcement: e.target.value })} />
        <p className="text-xs text-muted-foreground">{t('settings.announcementHint')}</p>
      </div>
      <Button type="submit" disabled={pending}>
        {t('settings.save')}
      </Button>
    </form>
  )
}

function PasswordForm() {
  const { t } = useTranslation()
  const [oldPw, setOldPw] = useState('')
  const [newPw, setNewPw] = useState('')
  const [confirmPw, setConfirmPw] = useState('')
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  const me = useQuery({ queryKey: adminKeys.me, queryFn: adminApi.me })

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (newPw.length < 8) return setError(t('settings.passwordTooShort'))
    if (newPw !== confirmPw) return setError(t('settings.passwordMismatch'))
    setError('')
    setPending(true)
    try {
      const r = await adminApi.changePassword(oldPw, newPw)
      tokenStore.set(r.token)
      setOldPw('')
      setNewPw('')
      setConfirmPw('')
      toast.success(t('settings.passwordChanged'))
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setPending(false)
    }
  }

  return (
    <form onSubmit={(e) => void submit(e)} className="space-y-4">
      {/* 隐藏的用户名：让密码管理器知道修改的是哪个账号，从而正确更新已保存的密码 */}
      <input type="text" name="username" autoComplete="username" value={me.data?.username ?? ''} readOnly hidden />
      <div className="space-y-1.5">
        <Label htmlFor="old_password">{t('settings.oldPassword')}</Label>
        <Input id="old_password" type="password" autoComplete="current-password" value={oldPw} onChange={(e) => setOldPw(e.target.value)} />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="new_password">{t('settings.newPassword')}</Label>
        <Input id="new_password" type="password" autoComplete="new-password" value={newPw} onChange={(e) => setNewPw(e.target.value)} />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="confirm_password">{t('settings.confirmPassword')}</Label>
        <Input id="confirm_password" type="password" autoComplete="new-password" value={confirmPw} onChange={(e) => setConfirmPw(e.target.value)} />
      </div>
      {error && (
        <p role="alert" className="text-sm text-bad">
          {error}
        </p>
      )}
      <Button type="submit" disabled={pending}>
        {t('settings.password')}
      </Button>
    </form>
  )
}

function BackupPanel({ onImported }: { onImported: () => void }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const fileRef = useRef<HTMLInputElement>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)
  // 解析成功、等待用户确认的导入内容；关闭弹窗时保留，避免关闭动画中数字变为 0
  const [pendingImport, setPendingImport] = useState<{ data: unknown; count: number } | null>(null)
  const [importOpen, setImportOpen] = useState(false)

  const doExport = async () => {
    try {
      const data = await adminApi.exportData()
      downloadJson(`sss-export-${new Date().toISOString().slice(0, 10)}.json`, data)
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  const onFile = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return
    let parsed: unknown
    try {
      parsed = JSON.parse(await file.text())
    } catch {
      toast.error(t('settings.importInvalid'))
      return
    }
    const servers = (parsed as { servers?: unknown } | null)?.servers
    setPendingImport({ data: parsed, count: Array.isArray(servers) ? servers.length : 0 })
    setImportOpen(true)
  }

  const doImport = async () => {
    if (!pendingImport) return
    setImportOpen(false)
    try {
      const r = await adminApi.importData(pendingImport.data)
      // 等设置查询重新拉取完成后再让两个表单重新挂载，避免读到导入前的缓存值
      await qc.invalidateQueries()
      onImported()
      toast.success(t('settings.importDone', { count: r.servers }))
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  return (
    <div className="flex flex-wrap gap-2">
      <Button variant="outline" onClick={() => setConfirmOpen(true)}>
        {t('settings.export')}
      </Button>
      <Button variant="outline" onClick={() => fileRef.current?.click()}>
        {t('settings.import')}
      </Button>
      <input ref={fileRef} type="file" accept="application/json,.json" className="hidden" onChange={(e) => void onFile(e)} />
      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('settings.exportTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('settings.exportDesc')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void doExport()}>{t('settings.export')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      <AlertDialog open={importOpen} onOpenChange={setImportOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('settings.importTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('settings.importDesc', { count: pendingImport?.count ?? 0 })}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void doImport()}>{t('settings.importConfirm')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

// 设置页分区；当前分区保存在网址 ?tab= 中，便于刷新、收藏和直接跳转
const TABS = ['site', 'notify', 'account', 'backup'] as const
type Tab = (typeof TABS)[number]

function isTab(v: string | null): v is Tab {
  return (TABS as readonly (string | null)[]).includes(v)
}

export function SettingsPage() {
  const { t } = useTranslation()
  const q = useQuery({ queryKey: adminKeys.settings, queryFn: adminApi.settings })
  // 表单版本号：仅在导入备份后递增，使两个表单重新挂载以读取导入后的设置；
  // 单独保存某个表单时不递增，从而不会重置另一个表单里未保存的编辑
  const [formVersion, setFormVersion] = useState(0)
  const [params, setParams] = useSearchParams()
  const raw = params.get('tab')
  const tab: Tab = isTab(raw) ? raw : 'site'
  const changeTab = (v: string) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (v === 'site') next.delete('tab')
        else next.set('tab', v)
        return next
      },
      { replace: true },
    )
  const labels: Record<Tab, string> = {
    site: t('settings.site'),
    notify: t('notify.title'),
    account: t('settings.account'),
    backup: t('settings.backupTab'),
  }
  // 各分区始终挂载、仅隐藏非当前分区，切换标签页时不会丢失未保存的编辑
  const pane = 'space-y-6 data-[state=inactive]:hidden'
  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <h1 className="text-lg font-semibold">{t('settings.title')}</h1>
      <Tabs value={tab} onValueChange={changeTab}>
        <TabsList className="max-w-full overflow-x-auto">
          {TABS.map((k) => (
            <TabsTrigger key={k} value={k}>
              {labels[k]}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="site" forceMount className={pane}>
          <Card title={t('settings.site')}>
            {q.data ? (
              <SiteSettingsForm key={formVersion} initial={q.data} />
            ) : q.error ? (
              <p className="text-sm text-bad">{errorMessage(q.error)}</p>
            ) : (
              <Skeleton className="h-48" />
            )}
          </Card>
        </TabsContent>
        <TabsContent value="notify" forceMount className={pane}>
          <Card title={t('notify.title')} desc={t('notify.desc')}>
            {q.data ? <NotifySettingsForm key={formVersion} initial={q.data} /> : <Skeleton className="h-64" />}
          </Card>
        </TabsContent>
        <TabsContent value="account" forceMount className={pane}>
          <Card title={t('settings.password')}>
            <PasswordForm />
          </Card>
        </TabsContent>
        <TabsContent value="backup" forceMount className={pane}>
          <Card title={t('settings.backup')} desc={t('settings.backupDesc')}>
            <BackupPanel onImported={() => setFormVersion((v) => v + 1)} />
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
