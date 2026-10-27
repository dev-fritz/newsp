import { useTranslation } from "react-i18next";

import { useWindowLabel } from "../hooks/use-window-label";
import type { ReportSummary } from "../types";

/** A report that is being generated: pulse, window/focus and an indeterminate bar. */
export function ReportProgressCard({ report }: { report: ReportSummary }) {
  const { t } = useTranslation("reports");
  const windowLabel = useWindowLabel()(report.window_start, report.window_end);
  const title = report.focus
    ? t("progress.titleWithFocus", { window: windowLabel, focus: report.focus })
    : t("progress.title", { window: windowLabel });

  return (
    <output
      aria-live="polite"
      className="flex flex-col gap-2 rounded-[10px] border border-dashed border-border-strong p-3"
    >
      <div className="flex items-center gap-2 text-[13px] font-medium">
        <span aria-hidden className="size-[7px] shrink-0 animate-pulse rounded-full bg-signal" />
        <span className="min-w-0 break-words">{title}</span>
      </div>
      <div aria-hidden className="h-1 overflow-hidden rounded-sm bg-muted">
        <div className="h-full w-2/5 animate-pulse rounded-sm bg-signal" />
      </div>
      <span className="text-xs text-subtle-foreground">
        {t(report.status === "running" ? "progress.running" : "progress.pending")}
      </span>
    </output>
  );
}
