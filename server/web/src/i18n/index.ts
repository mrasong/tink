import { en } from "./en";
import { zh } from "./zh";
import { getLocale } from "./state.svelte";

// 轻量 i18n：t(key, params) 读取响应式 locale（.svelte.ts $state），在模板中调用可随语言切换自动重渲染。
// 复数：传 params.count，键按 `${key}.one` / `${key}.other` 查找（zh 恒用 .other，可省略）。

function lookup(locale: "en" | "zh", key: string): string | undefined {
  const dict = locale === "zh" ? zh : en;
  return dict[key] ?? (locale === "zh" ? en[key] : undefined);
}

export function t(key: string, params?: Record<string, string | number>): string {
  const locale = getLocale();
  let template: string | undefined;
  if (params && typeof params.count === "number") {
    const suffix = locale === "zh" || params.count !== 1 ? "other" : "one";
    template = lookup(locale, `${key}.${suffix}`);
  }
  if (template === undefined) template = lookup(locale, key);
  if (template === undefined) return key;
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (_, name: string) => String(params[name] ?? ""));
}

export { getLocale, setLocale, toggleLocale, initI18n } from "./state.svelte";
export type { Locale } from "./locale";
