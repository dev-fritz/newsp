import { AlertCircle, Trash2 } from "lucide-react";
import { useState } from "react";
import { useLocation, useNavigate } from "react-router";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";

import { windowHours } from "../format";
import type { Report } from "../types";
import { DeleteReportDialog } from "./delete-report-dialog";
import { GenerateReportDialog } from "./generate-report-dialog";

/** A failed report: the translated reason, a way to try again and to delete it. */
export function ReportFailed({ report }: { report: Report }) {
  const { t } = useTranslation("reports");
  const navigate = useNavigate();
  const { search } = useLocation();
  const [deleting, setDeleting] = useState(false);
  return (
    <div className="mx-auto flex max-w-[720px] flex-col gap-4 px-5 pt-10 pb-16 sm:px-10 sm:pt-16">
      <div
        role="alert"
        className="flex flex-col items-start gap-3 rounded-2xl border border-border p-6 sm:p-8"
      >
        <AlertCircle className="size-5 text-negative" aria-hidden />
        <h1 className="font-serif text-2xl font-medium">{t("reader.failed.title")}</h1>
        <p className="text-muted-foreground">
          {t([`errors.${report.error ?? ""}`, "errors.generic"])}
        </p>
        <div className="flex flex-wrap gap-2">
          <GenerateReportDialog
            defaults={{
              windowHours: windowHours(report.window_start, report.window_end),
              focus: report.focus,
            }}
            onGenerated={() => void navigate("/reports")}
          >
            <Button type="button" className="h-9 pointer-coarse:h-11">
              {t("generate.retry")}
            </Button>
          </GenerateReportDialog>
          <Button
            type="button"
            variant="ghost"
            className="h-9 pointer-coarse:h-11"
            onClick={() => setDeleting(true)}
          >
            <Trash2 aria-hidden />
            {t("delete.button")}
          </Button>
        </div>
        {report.id && (
          <DeleteReportDialog
            reportId={report.id}
            open={deleting}
            onOpenChange={setDeleting}
            onDeleted={() => void navigate({ pathname: "/reports", search })}
          />
        )}
      </div>
    </div>
  );
}
