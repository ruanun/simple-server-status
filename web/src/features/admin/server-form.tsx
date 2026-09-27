import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import { cloneElement, isValidElement, type ReactElement, type ReactNode } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { adminApi, adminKeys } from '@/lib/admin-api'
import { errorMessage } from '@/lib/api'
import type { AdminServer } from '@/lib/types'

import { formSchema, fromServer, toInput, type FormValues } from './form-model'

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <fieldset className="space-y-3">
      <legend className="text-sm font-medium">{title}</legend>
      <div className="grid gap-3 sm:grid-cols-2">{children}</div>
    </fieldset>
  )
}

type A11yProps = { 'aria-invalid'?: boolean; 'aria-describedby'?: string }

/** Field 表单字段：错误或提示文字通过 aria-describedby 关联到输入框，出错时标记 aria-invalid */
function Field({ id, label, error, hint, children }: { id: string; label: string; error?: string; hint?: string; children: ReactElement<A11yProps> }) {
  const { t } = useTranslation()
  const noteId = error ? `${id}-error` : hint ? `${id}-hint` : undefined
  const control = isValidElement(children) ? cloneElement(children, { 'aria-invalid': error ? true : undefined, 'aria-describedby': noteId }) : children
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>{label}</Label>
      {control}
      {error ? (
        <p id={noteId} className="text-xs text-bad">
          {t(error)}
        </p>
      ) : hint ? (
        <p id={noteId} className="text-xs text-muted-foreground">
          {hint}
        </p>
      ) : null}
    </div>
  )
}

interface Props {
  server: AdminServer | null
  onClose: () => void
  onSaved: (saved: AdminServer, created: boolean) => void
}

/** ServerFormDialog 新建 / 编辑服务器，字段分为基本、计费、流量、采集四组 */
export function ServerFormDialog({ server, onClose, onSaved }: Props) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const {
    register,
    control,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(formSchema), defaultValues: fromServer(server) })

  const submit = handleSubmit(async (values) => {
    try {
      const input = toInput(values)
      const saved = server ? await adminApi.update(server.id, input) : await adminApi.create(input)
      await qc.invalidateQueries({ queryKey: adminKeys.servers })
      toast.success(t('admin.saved'))
      onSaved(saved, server === null)
    } catch (err) {
      toast.error(errorMessage(err))
    }
  })

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-2xl" aria-describedby={undefined}>
        <DialogHeader>
          <DialogTitle>{server ? t('form.editTitle') : t('form.createTitle')}</DialogTitle>
        </DialogHeader>
        <form onSubmit={(e) => void submit(e)} className="space-y-6" noValidate>
          <Section title={t('form.basic')}>
            <Field id="name" label={t('form.name')} error={errors.name?.message}>
              <Input id="name" {...register('name')} />
            </Field>
            <Field id="group" label={t('form.group')}>
              <Input id="group" {...register('group')} />
            </Field>
            <Field id="country" label={t('form.country')} error={errors.country?.message} hint={t('form.countryHint')}>
              <Input id="country" maxLength={2} {...register('country')} />
            </Field>
            <div className="flex items-center gap-2 sm:self-center">
              <Controller control={control} name="hidden" render={({ field }) => <Switch id="hidden" checked={field.value} onCheckedChange={field.onChange} />} />
              <Label htmlFor="hidden">{t('form.hidden')}</Label>
            </div>
          </Section>

          <Section title={t('form.billing')}>
            <Field id="price" label={t('form.price')} error={errors.price?.message}>
              <Input id="price" inputMode="decimal" {...register('price')} />
            </Field>
            <Field id="currency" label={t('form.currency')}>
              <Input id="currency" placeholder="$ / ¥ / USD" {...register('currency')} />
            </Field>
            <Field id="billing_cycle" label={t('form.cycle')}>
              <Controller
                control={control}
                name="billing_cycle"
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger id="billing_cycle" className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">{t('form.cycleNone')}</SelectItem>
                      {(['monthly', 'quarterly', 'yearly', 'once'] as const).map((c) => (
                        <SelectItem key={c} value={c}>
                          {t(`billing.${c}`)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
            </Field>
            <Field id="expire_at" label={t('form.expireAt')}>
              <Input id="expire_at" type="date" {...register('expire_at')} />
            </Field>
          </Section>

          <Section title={t('form.traffic')}>
            <Field id="traffic_limit_gb" label={t('form.trafficLimit')} error={errors.traffic_limit_gb?.message} hint={t('form.trafficLimitHint')}>
              <Input id="traffic_limit_gb" inputMode="decimal" {...register('traffic_limit_gb')} />
            </Field>
            <Field id="traffic_mode" label={t('form.trafficMode')}>
              <Controller
                control={control}
                name="traffic_mode"
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger id="traffic_mode" className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="sum">{t('form.modeSum')}</SelectItem>
                      <SelectItem value="in">{t('form.modeIn')}</SelectItem>
                      <SelectItem value="out">{t('form.modeOut')}</SelectItem>
                    </SelectContent>
                  </Select>
                )}
              />
            </Field>
            <Field id="traffic_reset_day" label={t('form.resetDay')} error={errors.traffic_reset_day?.message}>
              <Input id="traffic_reset_day" inputMode="numeric" {...register('traffic_reset_day')} />
            </Field>
          </Section>

          <Section title={t('form.collect')}>
            <Field id="report_interval" label={t('form.interval')} error={errors.report_interval?.message} hint={t('form.intervalHint')}>
              <Input id="report_interval" inputMode="numeric" {...register('report_interval')} />
            </Field>
            <Field id="nic_include" label={t('form.nicInclude')} hint={t('form.listHint')}>
              <Input id="nic_include" placeholder="eth0, ens" {...register('nic_include')} />
            </Field>
            <Field id="nic_exclude" label={t('form.nicExclude')} hint={t('form.listHint')}>
              <Input id="nic_exclude" {...register('nic_exclude')} />
            </Field>
            <Field id="mount_exclude" label={t('form.mountExclude')} hint={t('form.listHint')}>
              <Input id="mount_exclude" placeholder="/boot" {...register('mount_exclude')} />
            </Field>
          </Section>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              {t('form.cancel')}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {t('form.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
