import { ChevronLeft, Copy, Cpu, Sparkles, Trash2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation, useNavigate } from "react-router";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useIdeaGeneration } from "@/features/ideas";

import { formatTime } from "../format";
import { useReportIdeaBase } from "../hooks/use-report-idea-base";
import { isActive, type Report } from "../types";
import { DeleteReportDialog } from "./delete-report-dialog";
import { KindBadge } from "./kind-badge";

/** Sticky bar of the reader: kind/time and model badges, copy and "generate ideas". */
export function ReportToolbar({ report }: { report: Report }) {
  const { t, i18n } = useTranslation("reports");
  const { openDialog } = useIdeaGeneration();
  const base = useReportIdeaBase(report);
  const navigate = useNavigate();
  const { search } = useLocation();
  const [deleting, setDeleting] = useState(false);
  const active = isActive(report);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(report.content_md ?? "");
      toast.success(t("reader.copied"));
    } catch {
      toast.error(t("reader.copyFailed"));
    }
  };

  return (
    <div
      role="toolbar"
      aria-label={t("reader.toolbar")}
      className="sticky top-[57px] z-10 flex flex-wrap items-center justify-between gap-x-3 gap-y-2 border-b border-border bg-card/95 px-3 py-2 backdrop-blur sm:px-8 lg:top-0 lg:py-3.5 xl:rounded-t-2xl"
    >
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <Button asChild variant="ghost" className="-ml-1 h-11 px-2 xl:hidden">
          <Link to="/reports" aria-label={t("reader.back")}>
            <ChevronLeft aria-hidden />
            <span className="hidden sm:inline">{t("reader.back")}</span>
          </Link>
        </Button>
        <KindBadge kind={report.kind} />
        <span className="hidden font-mono text-[11px] text-subtle-foreground sm:inline">
          {formatTime(report.completed_at ?? report.created_at, i18n.language)}
        </span>
        {report.model_name && (
          <Badge
            variant="outline"
            className="hidden h-5 max-w-[240px] rounded-md text-[11px] text-muted-foreground sm:inline-flex"
            title={t("reader.model")}
          >
            <Cpu aria-hidden />
            <span className="truncate">{report.model_name}</span>
          </Badge>
        )}
      </div>
      <div className="flex items-center gap-1.5">
        <Button
          type="button"
          variant="ghost"
          onClick={copy}
          aria-label={t("reader.copyMarkdown")}
          className="h-9 pointer-coarse:h-11 pointer-coarse:min-w-11"
        >
          <Copy aria-hidden />
          <span className="hidden sm:inline">{t("reader.copyMarkdown")}</span>
        </Button>
        <Button
          type="button"
          variant="ghost"
          onClick={() => setDeleting(true)}
          disabled={active}
          title={active ? t("delete.inProgress") : undefined}
          aria-label={t("delete.button")}
          aria-haspopup="dialog"
          className="h-9 pointer-coarse:h-11 pointer-coarse:min-w-11"
        >
          <Trash2 aria-hidden />
          <span className="hidden sm:inline">{t("delete.button")}</span>
        </Button>
        <Button
          type="button"
          variant="secondary"
          className="hidden h-9 sm:inline-flex"
          aria-haspopup="dialog"
          disabled={!base}
          onClick={() => base && openDialog(base)}
        >
          <Sparkles aria-hidden />
          {t("reader.generateIdeas")}
        </Button>
      </div>
      {report.id && (
        <DeleteReportDialog
          reportId={report.id}
          open={deleting}
          onOpenChange={setDeleting}
          onDeleted={() => navigate({ pathname: "/reports", search })}
        />
      )}
    </div>
  );
}
