import { createI18n } from 'vue-i18n'
import zhCN from './zh-CN'
import en from './en'

const messages = {
  'zh-CN': zhCN,
  en: en,
}

// Get saved language or default to zh-CN
function getDefaultLocale() {
  const saved = localStorage.getItem('locale')
  if (saved && messages[saved]) return saved
  // Detect browser language
  const browserLang = navigator.language || navigator.userLanguage
  if (browserLang.startsWith('zh')) return 'zh-CN'
  return 'en'
}

const i18n = createI18n({
  legacy: false,
  locale: getDefaultLocale(),
  fallbackLocale: 'en',
  messages,
})

export default i18n

// Helper to change language
export function setLocale(locale) {
  i18n.global.locale.value = locale
  localStorage.setItem('locale', locale)
}

// Available locales
export const availableLocales = [
  { code: 'zh-CN', name: '中文', flag: '🇨🇳' },
  { code: 'en', name: 'English', flag: '🇺🇸' },
]
