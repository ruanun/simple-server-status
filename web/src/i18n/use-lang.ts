import { useTranslation } from 'react-i18next'

import type { Lang } from '@/lib/format'

/** useLang 当前界面语言（随切换重新渲染） */
export function useLang(): Lang {
  const { i18n } = useTranslation()
  return i18n.language === 'en-US' ? 'en-US' : 'zh-CN'
}
