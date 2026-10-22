import { createI18n } from 'vue-i18n'
import zhCN from './zh-CN'
import en from './en'
import zhTW from './zh-TW'
import ru from './ru'
import fa from './fa'
import fr from './fr'
import es from './es'
import ar from './ar'
import de from './de'
import ja from './ja'
import ko from './ko'
import pt from './pt'
import tr from './tr'
import id from './id'
import th from './th'
import vi from './vi'

const messages = {
  'zh-CN': zhCN,
  en: en,
  'zh-TW': zhTW,
  ru: ru,
  fa: fa,
  fr: fr,
  es: es,
  ar: ar,
  de: de,
  ja: ja,
  ko: ko,
  pt: pt,
  tr: tr,
  id: id,
  th: th,
  vi: vi,
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
  // 后台是「固定左侧栏 + 右侧内容」布局，整体镜像会让侧栏（菜单链接）翻到右边、
  // 顶栏内容翻到左边，与其它语言不一致 ✗ → 始终 LTR，只切 lang（字体/断词用）。
  document.documentElement.setAttribute('dir', 'ltr')
  document.documentElement.setAttribute('lang', locale)
}

// Available locales
export const availableLocales = [
  { code: 'zh-CN', name: '中文（简体）', flag: '🇨🇳' },
  { code: 'zh-TW', name: '中文（繁體）', flag: '🇨🇳' },
  { code: 'en', name: 'English', flag: '🇺🇸' },
  { code: 'ru', name: 'Русский', flag: '🇷🇺' },
  { code: 'fa', name: 'فارسی', flag: '🇮🇷' },
  { code: 'ar', name: 'العربية', flag: '🇸🇦' },
  { code: 'fr', name: 'Français', flag: '🇫🇷' },
  { code: 'es', name: 'Español', flag: '🇪🇸' },
  { code: 'de', name: 'Deutsch', flag: '🇩🇪' },
  { code: 'pt', name: 'Português', flag: '🇵🇹' },
  { code: 'ja', name: '日本語', flag: '🇯🇵' },
  { code: 'ko', name: '한국어', flag: '🇰🇷' },
  { code: 'tr', name: 'Türkçe', flag: '🇹🇷' },
  { code: 'th', name: 'ไทย', flag: '🇹🇭' },
  { code: 'vi', name: 'Tiếng Việt', flag: '🇻🇳' },
  { code: 'id', name: 'Bahasa Indonesia', flag: '🇮🇩' },
]

// Languages written right-to-left（仅用于文案层面的标记，后台布局不做镜像）
export const RTL_LOCALES = ['fa', 'ar']

// 后台布局始终 LTR：这里只写 lang 属性（首次加载同样不设 dir=rtl）
document.documentElement.setAttribute('lang', getDefaultLocale())
