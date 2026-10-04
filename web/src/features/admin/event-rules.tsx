import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import type { EventSettings, Settings } from '@/lib/types'

import { LabeledNumber } from './labeled-number'
import { useSaveSettings } from './use-save-settings'

/** EventRulesForm 检测规则：决定是否记录事件；保存时只覆盖检测规则 */
export function EventRulesForm({ initial }: { initial: Settings }) {
  const { t } = useTranslation()
  const [ev, setEv] = useState<EventSettings>(initial.events)
  const { pending, save } = useSaveSettings(initial)

  const field = (key: keyof EventSettings, label: string, min: number, max: number) => (
    <LabeledNumber id={`rules_${key}`} label={label} value={ev[key]} min={min} max={max} onChange={(v) => setEv({ ...ev, [key]: v })} />
  )

  const submit = (e: FormEvent) => {
    e.preventDefault()
    void save({ events: ev })
  }

  return (
    <form onSubmit={submit} className="space-y-4">
      <div className="space-y-2 rounded-md border p-3">
        <p className="text-sm font-medium">{t('rules.load')}</p>
        <div className="flex flex-wrap items-center gap-3 text-sm">
          {field('load_cpu', t('rules.loadCpu'), 1, 100)}
          {field('load_mem', t('rules.loadMem'), 1, 100)}
          {field('load_disk', t('rules.loadDisk'), 1, 100)}
          {field('load_minutes', t('rules.loadMinutes'), 1, 10)}
        </div>
      </div>
      <Button type="submit" disabled={pending}>
        {t('rules.save')}
      </Button>
    </form>
  )
}
