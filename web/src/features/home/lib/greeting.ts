export type DayPart = "morning" | "afternoon" | "evening";

export function dayPart(date: Date): DayPart {
  const hour = date.getHours();
  if (hour < 5) return "evening";
  if (hour < 12) return "morning";
  if (hour < 18) return "afternoon";
  return "evening";
}

/** "segunda-feira, 26 de outubro". */
export function formatKickerDate(date: Date, locale: string): string {
  const text = new Intl.DateTimeFormat(locale, {
    weekday: "long",
    day: "numeric",
    month: "long",
  }).format(date);
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/** First word of a display name. */
export function firstName(name: string): string {
  return name.trim().split(/\s+/)[0] ?? "";
}
