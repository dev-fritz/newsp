import { Link } from "react-router";
import { useTranslation } from "react-i18next";

import { EmptyState, ErrorState } from "@/components/common/states";
import { Button } from "@/components/ui/button";

import { useReport } from "../api/reports-queries";
import { ReportArticle } from "./report-article";
import { ReportFailed } from "./report-failed";
import { ReportGenerating } from "./report-generating";
import { ReportReaderSkeleton } from "./report-reader-skeleton";
import { ReportToolbar } from "./report-toolbar";

/** The reader for `/reports/:id`: toolbar + article, or the loading/404/error/failed state. */
export function ReportReader({ id }: { id: string }) {
  const { t } = useTranslation("reports");
  const query = useReport(id);

  if (query.isPending) return <ReportReaderSkeleton />;

  if (query.isError) {
    if (query.error.status === 404) {
      return (
        <div className="p-5 sm:p-8">
          <EmptyState
            title={t("reader.notFound.title")}
            body={t("reader.notFound.body")}
            action={
              <Button asChild variant="outline" className="h-9 pointer-coarse:h-11">
                <Link to="/reports">{t("reader.notFound.back")}</Link>
              </Button>
            }
          />
        </div>
      );
    }
    return (
      <div className="p-5 sm:p-8">
        <ErrorState error={query.error} onRetry={() => void query.refetch()} />
      </div>
    );
  }

  const report = query.data;
  if (report.status === "failed") return <ReportFailed report={report} />;
  if (report.status === "pending" || report.status === "running") {
    return <ReportGenerating report={report} />;
  }
  return (
    <>
      <ReportToolbar report={report} />
      <ReportArticle report={report} />
    </>
  );
}
