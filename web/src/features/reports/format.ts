import type { Citation } from "./types";

const HOUR_MS = 3_600_000;
const DAY_MS = 24 * HOUR_MS;
const WORDS_PER_MINUTE = 220;

function parse(iso: string | undefined): Date | null {
  if (!iso) return null;
  const date = new Date(iso);
  return Number.isNaN(date.getTime()) ? null : date;
}

function stripDot(text: string): string {
  return text.replace(/\.$/, "");
}

/** "SEG · 26 OUT" (mono label of a list row). */
export function formatShortDate(iso: string | undefined, locale: string): string {
  const date = parse(iso);
  if (!date) return "";
  const parts = new Intl.DateTimeFormat(locale, {
    weekday: "short",
    day: "numeric",
    month: "short",
  }).formatToParts(date);
  const pick = (type: Intl.DateTimeFormatPartTypes) =>
    stripDot(parts.find((part) => part.type === type)?.value ?? "");
  return `${pick("weekday")} · ${pick("day")} ${pick("month")}`.toUpperCase();
}

/** "segunda-feira, 26 de outubro" (kicker of the article). */
export function formatLongDate(iso: string | undefined, locale: string): string {
  const date = parse(iso);
  if (!date) return "";
  return new Intl.DateTimeFormat(locale, { weekday: "long", day: "numeric", month: "long" }).format(
    date,
  );
}

/** "25 out 08:00 → 26 out 08:00". */
export function formatWindow(
  start: string | undefined,
  end: string | undefined,
  locale: string,
): string {
  const from = parse(start);
  const to = parse(end);
  if (!from || !to) return "";
  const fmt = new Intl.DateTimeFormat(locale, {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
  return `${fmt.format(from)} → ${fmt.format(to)}`;
}

export function formatTime(iso: string | undefined, locale: string): string {
  const date = parse(iso);
  if (!date) return "";
  return new Intl.DateTimeFormat(locale, { hour: "2-digit", minute: "2-digit" }).format(date);
}

/** Length of the window a report covers, in whole hours. */
export function windowHours(start: string | undefined, end: string | undefined): number {
  const from = parse(start);
  const to = parse(end);
  if (!from || !to) return 0;
  return Math.max(1, Math.round((to.getTime() - from.getTime()) / HOUR_MS));
}

/** "há 14 h", "ontem"... */
export function formatRelative(iso: string | undefined, locale: string, now = Date.now()): string {
  const date = parse(iso);
  if (!date) return "";
  const diff = date.getTime() - now;
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto", style: "short" });
  const abs = Math.abs(diff);
  if (abs < HOUR_MS) return rtf.format(Math.round(diff / 60_000), "minute");
  if (abs < DAY_MS) return rtf.format(Math.round(diff / HOUR_MS), "hour");
  if (abs < 30 * DAY_MS) return rtf.format(Math.round(diff / DAY_MS), "day");
  return new Intl.DateTimeFormat(locale, {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(date);
}

/** "1,2 mil" / "1.2K". */
export function formatCompact(value: number, locale: string): string {
  return new Intl.NumberFormat(locale, { notation: "compact", maximumFractionDigits: 1 }).format(
    value,
  );
}

/** Reading time in minutes (at least 1), from the Markdown word count. */
export function readingMinutes(markdown: string | undefined): number {
  const words = markdown?.trim().split(/\s+/).filter(Boolean).length ?? 0;
  return Math.max(1, Math.ceil(words / WORDS_PER_MINUTE));
}

/** Only http(s) links from the API are ever rendered as links. */
export function safeHttpUrl(url: string | undefined): string | null {
  if (!url) return null;
  try {
    const parsed = new URL(url);
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? parsed.href : null;
  } catch {
    return null;
  }
}

const SOURCE_TAGS: Record<string, string> = {
  hackernews: "HN",
  tabnews: "TN",
  reddit: "r/",
  youtube: "YT",
  rss: "RSS",
  lobsters: "LO",
  devto: "DEV",
  mastodon: "M",
  bluesky: "BS",
  github: "GH",
  producthunt: "PH",
};

/** The tiny masthead tag of a source kind. */
export function sourceTag(kind: string | undefined): string {
  if (!kind) return "—";
  return SOURCE_TAGS[kind] ?? kind.slice(0, 3).toUpperCase();
}

export function engagementParts(citation: Citation): { score?: number; comments?: number } {
  return {
    score: typeof citation.score === "number" ? citation.score : undefined,
    comments: typeof citation.comments === "number" ? citation.comments : undefined,
  };
}

function startOfWeek(date: Date): number {
  const copy = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const sinceMonday = (copy.getDay() + 6) % 7;
  copy.setDate(copy.getDate() - sinceMonday);
  return copy.getTime();
}

export type ReportGroup<T> = {
  key: string;
  label: { kind: "thisWeek" | "lastWeek" } | { kind: "month"; text: string };
  items: T[];
};

/** Groups items (newest first) into "this week", "last week" and one group per older month. */
export function groupByWeek<T>(
  items: T[],
  dateOf: (item: T) => string | undefined,
  locale: string,
  now = new Date(),
): ReportGroup<T>[] {
  const thisWeek = startOfWeek(now);
  const lastWeek = thisWeek - 7 * DAY_MS;
  const monthFormat = new Intl.DateTimeFormat(locale, { month: "long", year: "numeric" });
  const groups: ReportGroup<T>[] = [];
  for (const item of items) {
    const date = parse(dateOf(item)) ?? now;
    const week = startOfWeek(date);
    let key: string;
    let label: ReportGroup<T>["label"];
    if (week >= thisWeek) {
      key = "thisWeek";
      label = { kind: "thisWeek" };
    } else if (week >= lastWeek) {
      key = "lastWeek";
      label = { kind: "lastWeek" };
    } else {
      key = `${date.getFullYear()}-${date.getMonth()}`;
      const text = monthFormat.format(date);
      label = { kind: "month", text: text.charAt(0).toUpperCase() + text.slice(1) };
    }
    const last = groups[groups.length - 1];
    if (last && last.key === key) last.items.push(item);
    else groups.push({ key, label, items: [item] });
  }
  return groups;
}
