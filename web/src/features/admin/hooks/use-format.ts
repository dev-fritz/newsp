import { useTranslation } from "react-i18next";

import {
  formatDateTime,
  formatInterval,
  formatNumber,
  formatRelative,
  formatTokens,
} from "../lib/format";

/** Intl formatters bound to the current language. */
export function useFormat() {
  const { i18n } = useTranslation();
  const locale = i18n.language;
  return {
    relative: (value: string | Date) => formatRelative(value, locale),
    dateTime: (value: string | Date) => formatDateTime(value, locale),
    interval: (minutes: number) => formatInterval(minutes, locale),
    tokens: (count: number) => formatTokens(count, locale),
    number: (value: number, digits?: number) => formatNumber(value, locale, digits),
  };
}
