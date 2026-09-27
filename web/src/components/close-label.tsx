import { useTranslation } from 'react-i18next'

/** CloseLabel 对话框 / 抽屉关闭按钮的读屏文本（随界面语言切换） */
export function CloseLabel() {
  const { t } = useTranslation()
  return <span className="sr-only">{t('common.close')}</span>
}
