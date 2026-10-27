import { useTranslation } from "react-i18next";

import { useWindowLabel } from "../hooks/use-window-label";
import type { Report } from "../types";

/** Reader state for a report that is still pending/running (the page polls). */
export function ReportGenerating({ report }: { report: Report }) {
  const { t } = useTranslation("reports");
  const windowLabel = useWindowLabel()(report.window_start, report.window_end);
  return (
    <div className="mx-auto flex max-w-[720px] flex-col gap-4 px-5 pt-10 pb-16 sm:px-10 sm:pt-16">
      <output
        aria-live="polite"
        className="block flex flex-col items-start gap-3 rounded-2xl border border-dashed border-border-strong p-6 sm:p-8"
      >
        <span aria-hidden className="size-2 animate-pulse rounded-full bg-signal" />
        <h1 className="font-serif text-2xl font-medium">{t("reader.generating.title")}</h1>
        <p className="text-muted-foreground">{t("reader.generating.body")}</p>
        <p className="text-sm text-subtle-foreground">
          {t(report.status === "running" ? "progress.running" : "progress.pending")} · {windowLabel}
          {report.focus ? ` · “${report.focus}”` : ""}
        </p>
      </output>
    </div>
  );
}
