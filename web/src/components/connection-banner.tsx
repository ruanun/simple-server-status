import { useTranslation } from 'react-i18next'

export function ConnectionBanner() {
  const { t } = useTranslation()
  return <div className="border-b bg-warn/10 px-4 py-1.5 text-center text-xs">{t('conn.polling')}</div>
}
