import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { adminApi, adminKeys } from '@/lib/admin-api'
import { errorMessage } from '@/lib/api'
import type { Settings } from '@/lib/types'

type Patch = Partial<Settings> & { turnstile_token?: string }

/**
 * useSaveSettings 设置页各表单共用的保存：以缓存中最新的设置为基础只覆盖 patch 中的字段，
 * 避免吞掉其他表单已保存的内容；保存后直接更新缓存而不让表单重新挂载，其他表单中未保存的编辑得以保留。
 * save 成功时提示并返回保存后的设置，失败时提示并返回 null
 */
export function useSaveSettings(initial: Settings) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [pending, setPending] = useState(false)

  const save = async (patch: Patch): Promise<Settings | null> => {
    setPending(true)
    try {
      const latest = qc.getQueryData<Settings>(adminKeys.settings) ?? initial
      const saved = await adminApi.saveSettings({ ...latest, ...patch })
      qc.setQueryData(adminKeys.settings, saved)
      toast.success(t('settings.saved'))
      return saved
    } catch (err) {
      toast.error(errorMessage(err))
      return null
    } finally {
      setPending(false)
    }
  }
  return { pending, save }
}
