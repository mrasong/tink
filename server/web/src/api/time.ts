import { t } from "@/i18n";

export function formatTimestamp(
  value: number | undefined,
  emptyValue?: string,
): string {
  if (!value) return emptyValue ?? t("time.unknown");
  return new Date(value).toLocaleString("sv-SE");
}
