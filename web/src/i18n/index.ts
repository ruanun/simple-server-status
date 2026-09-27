import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import type { Lang } from '@/lib/format'

import { enUS } from './en-US'
import { zhCN } from './zh-CN'

const KEY = 'sss.locale'

/** detectLang 优先使用已保存的语言，否则按浏览器语言选择 */
export function detectLang(): Lang {
  const saved = localStorage.getItem(KEY)
  if (saved === 'zh-CN' || saved === 'en-US') return saved
  return navigator.language.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en-US'
}

void i18n.use(initReactI18next).init({
  resources: { 'zh-CN': { translation: zhCN }, 'en-US': { translation: enUS } },
  lng: detectLang(),
  fallbackLng: 'zh-CN',
  interpolation: { escapeValue: false },
})

i18n.on('languageChanged', (lng) => {
  document.documentElement.lang = lng
})

/** setLang 切换并保存界面语言 */
export function setLang(lang: Lang) {
  localStorage.setItem(KEY, lang)
  void i18n.changeLanguage(lang)
}

export default i18n
