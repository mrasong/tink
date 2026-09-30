export type Locale = 'en' | 'zh'

export const LOCALE_STORAGE_KEY = 'tink-locale'

export function detectInitialLocale(): Locale {
  const stored = localStorage.getItem(LOCALE_STORAGE_KEY)
  if (stored === 'en' || stored === 'zh') return stored
  // 无偏好时按浏览器语言嗅探，zh* 给中文，其余兜底英文
  return (navigator.language || 'en').toLowerCase().startsWith('zh') ? 'zh' : 'en'
}

export function applyLocaleToDocument(locale: Locale) {
  document.documentElement.lang = locale === 'zh' ? 'zh-CN' : 'en'
}
