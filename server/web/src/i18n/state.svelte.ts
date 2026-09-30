import { detectInitialLocale, applyLocaleToDocument, LOCALE_STORAGE_KEY, type Locale } from "./locale";

export type { Locale };

let current = $state<Locale>(detectInitialLocale());

export function initI18n(): void {
  applyLocaleToDocument(current);
}

export function getLocale(): Locale {
  return current;
}

export function setLocale(locale: Locale) {
  if (locale === current) return;
  current = locale;
  localStorage.setItem(LOCALE_STORAGE_KEY, locale);
  applyLocaleToDocument(locale);
}

export function toggleLocale(): Locale {
  setLocale(current === "zh" ? "en" : "zh");
  return current;
}
