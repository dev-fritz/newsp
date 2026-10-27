import { MoreHorizontal, Trash2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation, useNavigate } from "react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

import { formatShortDate } from "../format";
import { useWindowLabel } from "../hooks/use-window-label";
import { isActive, type ReportSummary } from "../types";
import { DeleteReportDialog } from "./delete-report-dialog";
import { KindBadge } from "./kind-badge";

type ReportListItemProps = {
  report: ReportSummary;
  /** Highlights the row (the report open in the reader). */
  selected: boolean;
};

/** One row of the list: mono date, kind badge and serif title, plus a delete action menu. */
export function ReportListItem({ report, selected }: ReportListItemProps) {
  const { t, i18n } = useTranslation("reports");
  const { search } = useLocation();
  const navigate = useNavigate();
  const [deleting, setDeleting] = useState(false);
  const windowLabel = useWindowLabel()(report.window_start, report.window_end);
  const failed = report.status === "failed";
  const detail = [report.kind === "manual" ? windowLabel : "", report.focus && `“${report.focus}”`]
    .filter(Boolean)
    .join(" · ");
  const title =
    report.title ||
    (failed ? t(`errors.${report.error}`, { defaultValue: t("errors.generic") }) : "");

  return (
    <div className="group relative">
      <Link
        to={{ pathname: `/reports/${report.id}`, search }}
        aria-current={selected ? "true" : undefined}
        className={cn(
          "flex min-h-11 flex-col gap-1.5 rounded-[10px] border border-transparent p-3 text-foreground transition-colors hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
          selected && "border-border bg-card shadow-xs hover:bg-card",
        )}
      >
        <span className="flex items-center justify-between gap-2 pr-8 pointer-coarse:pr-11">
          <span
            className={cn(
              "font-mono text-xs font-medium",
              selected ? "text-foreground" : "text-subtle-foreground",
            )}
          >
            {formatShortDate(report.window_end ?? report.created_at, i18n.language)}
          </span>
          <span className="flex items-center gap-1.5">
            {failed && (
              <Badge
                variant="secondary"
                className="h-5 rounded-md bg-negative-soft text-[11px] text-negative"
              >
                {t("status.failed")}
              </Badge>
            )}
            <KindBadge kind={report.kind} />
          </span>
        </span>
        <span
          className={cn(
            "font-serif text-[16.5px] leading-[1.3] text-pretty",
            selected && "font-medium",
            failed && "text-muted-foreground",
          )}
        >
          {title}
        </span>
        {detail && <span className="text-xs text-subtle-foreground">{detail}</span>}
      </Link>
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={t("delete.actions")}
            className="absolute top-1.5 right-1.5 pointer-coarse:size-11 pointer-fine:opacity-0 pointer-fine:group-focus-within:opacity-100 pointer-fine:group-hover:opacity-100 pointer-fine:data-[state=open]:opacity-100"
          >
            <MoreHorizontal aria-hidden />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem
            variant="destructive"
            disabled={isActive(report)}
            onSelect={() => setDeleting(true)}
          >
            <Trash2 aria-hidden />
            {t("delete.menu")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      {report.id && (
        <DeleteReportDialog
          reportId={report.id}
          open={deleting}
          onOpenChange={setDeleting}
          onDeleted={() => {
            if (selected) navigate({ pathname: "/reports", search });
          }}
        />
      )}
    </div>
  );
}
