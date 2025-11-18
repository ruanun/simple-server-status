/**
 * i18n 国际化配置
 * 支持中英文切换，根据浏览器语言自动检测
 * @author ruan
 */

import { createI18n } from 'vue-i18n'
import type { LocaleType } from './types'
import zhCN from './zh-CN'
import enUS from './en-US'

const LOCALE_STORAGE_KEY = 'app-locale'

/**
 * 检测浏览器语言
 * 中文优先：zh/zh-CN/zh-TW → zh-CN
 * 其他语言 → en-US
 */
function detectBrowserLanguage(): LocaleType {
  // 先检查 localStorage 中是否有保存的语言设置
  const savedLocale = localStorage.getItem(LOCALE_STORAGE_KEY)
  if (savedLocale === 'zh-CN' || savedLocale === 'en-US') {
    return savedLocale as LocaleType
  }

  // 获取浏览器语言
  const browserLang = navigator.language.toLowerCase()

  // 中文语言检测（zh, zh-cn, zh-tw, zh-hk 等）
  if (browserLang.startsWith('zh')) {
    return 'zh-CN'
  }

  // 默认使用英文
  return 'en-US'
}

/**
 * 创建 i18n 实例
 */
const i18n = createI18n({
  legacy: false, // 使用 Composition API 模式
  locale: detectBrowserLanguage(),
  fallbackLocale: 'en-US',
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS
  },
  globalInjection: true // 全局注入 $t 函数
})

/**
 * 切换语言
 * @param locale 目标语言
 */
export function setLocale(locale: LocaleType) {
  i18n.global.locale.value = locale
  localStorage.setItem(LOCALE_STORAGE_KEY, locale)

  // 更新 HTML lang 属性
  document.documentElement.lang = locale
}

/**
 * 获取当前语言
 */
export function getLocale(): LocaleType {
  return i18n.global.locale.value as LocaleType
}

export default i18n
