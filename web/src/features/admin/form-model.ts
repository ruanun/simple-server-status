import { z } from 'zod'

import type { AdminServer, BillingCycle, ServerInput } from '@/lib/types'

const DECIMAL = /^(\d+(\.\d+)?)?$/
const GB = 1024 ** 3

/** formSchema 服务器表单校验；错误信息为 i18n 键，由界面翻译 */
export const formSchema = z.object({
  name: z.string().trim().min(1, 'form.nameRequired').max(64, 'form.nameTooLong'),
  group: z.string(),
  country: z.string().trim().regex(/^([A-Za-z]{2})?$/, 'form.countryInvalid'),
  hidden: z.boolean(),
  price: z.string().trim().regex(DECIMAL, 'form.numberInvalid'),
  currency: z.string(),
  billing_cycle: z.enum(['none', 'monthly', 'quarterly', 'yearly', 'once']),
  expire_at: z.string(),
  traffic_limit_gb: z.string().trim().regex(DECIMAL, 'form.numberInvalid'),
  traffic_mode: z.enum(['sum', 'in', 'out']),
  traffic_reset_day: z
    .string()
    .trim()
    .refine((v) => /^\d+$/.test(v) && Number(v) >= 1 && Number(v) <= 28, 'form.resetDayRange'),
  report_interval: z
    .string()
    .trim()
    .refine((v) => v === '' || (/^\d+$/.test(v) && Number(v) >= 1 && Number(v) <= 60), 'form.intervalRange'),
  nic_include: z.string(),
  nic_exclude: z.string(),
  mount_exclude: z.string(),
})

export type FormValues = z.infer<typeof formSchema>

function pad(n: number) {
  return String(n).padStart(2, '0')
}

/** toDateInput Unix 秒 → 本地日期 yyyy-mm-dd */
function toDateInput(ts: number | null): string {
  if (ts == null) return ''
  const d = new Date(ts * 1000)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function splitList(s: string): string[] {
  return s
    .split(',')
    .map((x) => x.trim())
    .filter(Boolean)
}

export function fromServer(s: AdminServer | null): FormValues {
  return {
    name: s?.name ?? '',
    group: s?.group ?? '',
    country: s?.country ?? '',
    hidden: s?.hidden ?? false,
    price: s?.price != null ? String(s.price) : '',
    currency: s?.currency ?? '',
    billing_cycle: s?.billing_cycle ? s.billing_cycle : 'none',
    expire_at: toDateInput(s?.expire_at ?? null),
    traffic_limit_gb: s?.traffic_limit != null ? String(Math.round((s.traffic_limit / GB) * 100) / 100) : '',
    traffic_mode: s?.traffic_mode ?? 'sum',
    traffic_reset_day: String(s?.traffic_reset_day ?? 1),
    report_interval: s ? String(s.report_interval) : '',
    nic_include: (s?.nic_include ?? []).join(', '),
    nic_exclude: (s?.nic_exclude ?? []).join(', '),
    mount_exclude: (s?.mount_exclude ?? []).join(', '),
  }
}

/** toInput 表单值 → 后端提交数据；到期日按本地零点换算，配额 GB → 字节，间隔为空时传 0 由后端取默认 */
export function toInput(v: FormValues): ServerInput {
  return {
    name: v.name.trim(),
    group: v.group.trim(),
    country: v.country.trim().toUpperCase(),
    hidden: v.hidden,
    price: v.price ? Number(v.price) : null,
    currency: v.currency.trim(),
    billing_cycle: v.billing_cycle === 'none' ? '' : (v.billing_cycle as BillingCycle),
    expire_at: v.expire_at ? Math.floor(new Date(`${v.expire_at}T00:00:00`).getTime() / 1000) : null,
    traffic_limit: v.traffic_limit_gb ? Math.round(Number(v.traffic_limit_gb) * GB) : null,
    traffic_mode: v.traffic_mode,
    traffic_reset_day: Number(v.traffic_reset_day),
    report_interval: v.report_interval ? Number(v.report_interval) : 0,
    nic_include: splitList(v.nic_include),
    nic_exclude: splitList(v.nic_exclude),
    mount_exclude: splitList(v.mount_exclude),
  }
}
