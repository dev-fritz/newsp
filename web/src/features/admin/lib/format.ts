const MINUTE = 60;
const HOUR = 3600;
const DAY = 86400;

/** "3 minutes ago" / "in 3 minutes" with Intl; under a minute reads "now". */
export function formatRelative(value: string | Date, locale: string, now = Date.now()): string {
  const date = typeof value === "string" ? new Date(value) : value;
  const seconds = Math.round((date.getTime() - now) / 1000);
  const abs = Math.abs(seconds);
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
  if (abs < MINUTE) return rtf.format(0, "second");
  if (abs < HOUR) return rtf.format(Math.round(seconds / MINUTE), "minute");
  if (abs < DAY) return rtf.format(Math.round(seconds / HOUR), "hour");
  if (abs < 30 * DAY) return rtf.format(Math.round(seconds / DAY), "day");
  if (abs < 365 * DAY) return rtf.format(Math.round(seconds / (30 * DAY)), "month");
  return rtf.format(Math.round(seconds / (365 * DAY)), "year");
}

export function formatDateTime(value: string | Date, locale: string): string {
  const date = typeof value === "string" ? new Date(value) : value;
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(date);
}

/** 15 -> "15 min", 60 -> "1 h" (narrow units). */
export function formatInterval(minutes: number, locale: string): string {
  const hours = minutes % 60 === 0;
  return new Intl.NumberFormat(locale, {
    style: "unit",
    unit: hours ? "hour" : "minute",
    unitDisplay: "narrow",
  }).format(hours ? minutes / 60 : minutes);
}

export function formatTokens(count: number, locale: string): string {
  return new Intl.NumberFormat(locale, { notation: "compact", maximumFractionDigits: 1 }).format(
    count,
  );
}

export function formatNumber(value: number, locale: string, digits = 2): string {
  return new Intl.NumberFormat(locale, {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  }).format(value);
}
