import { useTranslation } from "react-i18next";

import type { ReportBase } from "@/features/ideas";

import { formatLongDate } from "../format";
import type { Report } from "../types";

/** What the idea generation dialog shows about a report; null while it has no id. */
export function useReportIdeaBase(report: Report): ReportBase | null {
  const { t, i18n } = useTranslation("reports");
  if (!report.id) return null;
  const meta = [
    t(report.kind === "manual" ? "kind.manual" : "kind.scheduled"),
    formatLongDate(report.window_end ?? report.created_at, i18n.language),
    t("reader.cited", { count: (report.citations ?? []).length }),
  ]
    .filter(Boolean)
    .join(" · ");
  return { id: report.id, title: report.title ?? "", meta };
}
