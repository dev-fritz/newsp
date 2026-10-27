import { Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { useIdeaGeneration } from "@/features/ideas";

import { formatLongDate, formatWindow, readingMinutes } from "../format";
import { useReportIdeaBase } from "../hooks/use-report-idea-base";
import type { Report } from "../types";
import { CitedSources } from "./cited-sources";
import { ReportMarkdown } from "./report-markdown";

/** A finished report: kicker, title, summary, meta line, content and cited sources. */
export function ReportArticle({ report }: { report: Report }) {
  const { t, i18n } = useTranslation("reports");
  const { openDialog } = useIdeaGeneration();
  const base = useReportIdeaBase(report);
  const citations = report.citations ?? [];
  const range = formatWindow(report.window_start, report.window_end, i18n.language);
  const kicker = [formatLongDate(report.window_end ?? report.created_at, i18n.language), range]
    .filter(Boolean)
    .join(" · ");

  return (
    <article className="mx-auto flex max-w-[720px] flex-col gap-7 px-5 pt-6 pb-16 sm:px-10 sm:pt-12 sm:pb-20">
      <header className="flex flex-col gap-3 sm:gap-3.5">
        <span className="kicker">{kicker}</span>
        <h1 className="font-serif text-[32px] leading-[1.1] font-medium text-balance sm:text-[44px] sm:leading-[1.08]">
          {report.title}
        </h1>
        {report.summary && (
          <p className="font-serif text-lg leading-normal text-muted-foreground italic sm:text-[21px] sm:leading-normal">
            {report.summary}
          </p>
        )}
        <p className="flex flex-wrap gap-x-4 gap-y-1 pt-1.5 text-[12.5px] text-subtle-foreground sm:text-[13px]">
          {typeof report.items_considered === "number" && (
            <span>{t("reader.itemsAnalyzed", { count: report.items_considered })}</span>
          )}
          <span>{t("reader.cited", { count: citations.length })}</span>
          <span>{t("reader.reading", { count: readingMinutes(report.content_md) })}</span>
        </p>
      </header>

      {report.content_md ? (
        <ReportMarkdown content={report.content_md} citations={citations} />
      ) : (
        <p className="text-muted-foreground">{t("reader.emptyContent")}</p>
      )}

      <CitedSources citations={citations} />

      <Button
        type="button"
        className="h-12 text-sm sm:hidden"
        aria-haspopup="dialog"
        disabled={!base}
        onClick={() => base && openDialog(base)}
      >
        <Sparkles aria-hidden />
        {t("reader.generateIdeas")}
      </Button>
    </article>
  );
}
