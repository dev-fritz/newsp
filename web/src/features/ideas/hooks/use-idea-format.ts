import { useTranslation } from "react-i18next";

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

export type IdeaFormat = {
  /** "2 h ago" for recent dates, a short date otherwise. */
  relative: (iso: string) => string;
  date: (iso: string) => string;
  dateTime: (iso: string) => string;
  decimal: (value: number) => string;
};

/** Date and number formatting in the active language. */
export function useIdeaFormat(): IdeaFormat {
  const { i18n } = useTranslation();
  const lng = i18n.language;
  const rtf = new Intl.RelativeTimeFormat(lng, { numeric: "auto", style: "short" });
  const shortDate = new Intl.DateTimeFormat(lng, { day: "numeric", month: "short" });
  const fullDate = new Intl.DateTimeFormat(lng, {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
  const full = new Intl.DateTimeFormat(lng, {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
  const decimals = new Intl.NumberFormat(lng, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });

  return {
    relative: (iso) => {
      const time = new Date(iso).getTime();
      if (Number.isNaN(time)) return "";
      const elapsed = Date.now() - time;
      if (elapsed < MINUTE) return rtf.format(0, "second");
      if (elapsed < HOUR) return rtf.format(-Math.floor(elapsed / MINUTE), "minute");
      if (elapsed < DAY) return rtf.format(-Math.floor(elapsed / HOUR), "hour");
      if (elapsed < 7 * DAY) return rtf.format(-Math.floor(elapsed / DAY), "day");
      return shortDate.format(time);
    },
    date: (iso) => (Number.isNaN(Date.parse(iso)) ? "" : fullDate.format(new Date(iso))),
    dateTime: (iso) => (Number.isNaN(Date.parse(iso)) ? "" : full.format(new Date(iso))),
    decimal: (value) => decimals.format(value),
  };
}
