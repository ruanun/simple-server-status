import { describe, expect, it } from 'vitest'

import { makeAdminServer } from '@/test/fixtures'

import { formSchema, fromServer, toInput } from './form-model'

describe('表单模型', () => {
  it('服务器 → 表单 → 提交数据往返一致', () => {
    const expire = Math.floor(new Date('2027-01-15T00:00:00').getTime() / 1000)
    const s = makeAdminServer({
      price: 9.9,
      currency: 'USD',
      billing_cycle: 'yearly',
      expire_at: expire,
      traffic_limit: 1024 ** 4,
      nic_include: ['eth0', 'ens'],
      report_interval: 5,
    })
    const v = fromServer(s)
    expect(v.traffic_limit_gb).toBe('1024')
    expect(v.expire_at).toBe('2027-01-15')
    expect(v.nic_include).toBe('eth0, ens')
    const input = toInput(v)
    expect(input).toMatchObject({ price: 9.9, billing_cycle: 'yearly', expire_at: expire, traffic_limit: 1024 ** 4, nic_include: ['eth0', 'ens'], report_interval: 5 })
  })

  it('新服务器使用默认值（间隔 0 表示由后端取默认）', () => {
    expect(toInput(fromServer(null))).toMatchObject({
      name: '',
      country: '',
      price: null,
      traffic_limit: null,
      billing_cycle: '',
      expire_at: null,
      traffic_reset_day: 1,
      report_interval: 0,
      nic_include: [],
    })
  })

  it('校验失败时返回 i18n 键', () => {
    const r = formSchema.safeParse({ ...fromServer(null), name: ' ', country: 'HKG', price: 'abc', traffic_reset_day: '29', report_interval: '61' })
    expect(r.success).toBe(false)
    const messages = r.success ? [] : r.error.issues.map((i) => i.message)
    expect(messages).toEqual(expect.arrayContaining(['form.nameRequired', 'form.countryInvalid', 'form.numberInvalid', 'form.resetDayRange', 'form.intervalRange']))
  })
})

describe('备注与静音', () => {
  it('来回转换并去除备注首尾空白', () => {
    const v = fromServer(makeAdminServer({ note: '备注', notify_muted: true }))
    expect(v.note).toBe('备注')
    expect(v.notify_muted).toBe(true)
    expect(toInput({ ...v, note: '  新备注  ' })).toMatchObject({ note: '新备注', notify_muted: true })
  })
  it('备注超过 2000 字符不通过校验', () => {
    const v = fromServer(null)
    expect(formSchema.safeParse({ ...v, name: 'a', note: 'x'.repeat(2001) }).success).toBe(false)
  })
})
