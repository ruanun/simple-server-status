import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useRef, useState, type ChangeEvent, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
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
import { adminApi, adminKeys } from '@/lib/admin-api'
import { errorMessage, tokenStore } from '@/lib/api'
import { downloadJson } from '@/lib/download'
import type { Settings } from '@/lib/types'

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

function SiteSettingsForm({ initial }: { initial: Settings }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [s, setS] = useState(initial)
  const [pending, setPending] = useState(false)

  const save = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    try {
      await adminApi.saveSettings({ ...s, default_report_interval: Number(s.default_report_interval) })
      await Promise.all([qc.invalidateQueries({ queryKey: adminKeys.settings }), qc.invalidateQueries({ queryKey: ['site'] })])
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

function BackupPanel() {
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
      await qc.invalidateQueries()
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

export function SettingsPage() {
  const { t } = useTranslation()
  const q = useQuery({ queryKey: adminKeys.settings, queryFn: adminApi.settings })
  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <h1 className="text-lg font-semibold">{t('settings.title')}</h1>
      <Card title={t('settings.site')}>
        {q.data ? (
          <SiteSettingsForm key={JSON.stringify(q.data)} initial={q.data} />
        ) : q.error ? (
          <p className="text-sm text-bad">{errorMessage(q.error)}</p>
        ) : (
          <Skeleton className="h-48" />
        )}
      </Card>
      <Card title={t('settings.password')}>
        <PasswordForm />
      </Card>
      <Card title={t('settings.backup')} desc={t('settings.backupDesc')}>
        <BackupPanel />
      </Card>
    </div>
  )
}
