import { useTranslation } from "react-i18next";

import type { FeedItem } from "../types";

const MINUTE_MS = 60_000;
const HOUR_MS = 3_600_000;
const DAY_MS = 24 * HOUR_MS;

export type ItemFormat = {
  /** "há 14 h", "ontem"… */
  relative: (iso: string | undefined) => string;
  /** "238 pts · 91 comentários" ("" when the item carries no engagement). */
  engagement: (item: FeedItem) => string;
};

/** Locale-aware formatters for the dates and engagement numbers of an item. */
export function useItemFormat(): ItemFormat {
  const { t, i18n } = useTranslation("feed");
  const locale = i18n.language;
  const compact = new Intl.NumberFormat(locale, { notation: "compact", maximumFractionDigits: 1 });

  return {
    relative: (iso) => {
      if (!iso) return "";
      const date = new Date(iso);
      if (Number.isNaN(date.getTime())) return "";
      const diff = date.getTime() - Date.now();
      const abs = Math.abs(diff);
      const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto", style: "short" });
      if (abs < HOUR_MS) return rtf.format(Math.round(diff / MINUTE_MS), "minute");
      if (abs < DAY_MS) return rtf.format(Math.round(diff / HOUR_MS), "hour");
      if (abs < 30 * DAY_MS) return rtf.format(Math.round(diff / DAY_MS), "day");
      return new Intl.DateTimeFormat(locale, {
        day: "numeric",
        month: "short",
        year: "numeric",
      }).format(date);
    },
    engagement: (item) => {
      const parts: string[] = [];
      const score = item.score ?? 0;
      const comments = item.comments ?? 0;
      if (score > 0) {
        parts.push(
          t([`engagement.score.${item.source?.kind ?? ""}`, "engagement.score.default"], {
            count: score,
            value: compact.format(score),
          }),
        );
      }
      if (comments > 0) {
        parts.push(t("engagement.comments", { count: comments, value: compact.format(comments) }));
      }
      return parts.join(" · ");
    },
  };
}
