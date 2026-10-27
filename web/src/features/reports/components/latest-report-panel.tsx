import { Plus } from "lucide-react";
import { useTranslation } from "react-i18next";

import { EmptyState, ErrorState } from "@/components/common/states";
import { Button } from "@/components/ui/button";

import { useReportsInfinite } from "../api/reports-queries";
import { useKindFilter } from "../hooks/use-kind-filter";
import { GenerateReportDialog } from "./generate-report-dialog";
import { ReportReader } from "./report-reader";
import { ReportReaderSkeleton } from "./report-reader-skeleton";

/** The id of the report shown at `/reports` (no id): the newest finished one in the list. */
export function useDefaultReportId(): string | undefined {
  const [kind] = useKindFilter();
  const query = useReportsInfinite(kind);
  return query.data?.pages.flatMap((page) => page.data).find((r) => r.status === "done")?.id;
}

/** Reader pane at `/reports`: the latest report, or an empty state. */
export function LatestReportPanel() {
  const { t } = useTranslation("reports");
  const [kind] = useKindFilter();
  const query = useReportsInfinite(kind);
  const latestId = useDefaultReportId();

  if (query.isPending) return <ReportReaderSkeleton />;
  if (query.isError) {
    return (
      <div className="p-5 sm:p-8">
        <ErrorState error={query.error} onRetry={() => void query.refetch()} />
      </div>
    );
  }
  if (latestId) return <ReportReader id={latestId} />;
  return (
    <div className="p-5 sm:p-8">
      <EmptyState
        title={t("reader.noneYet.title")}
        body={t("reader.noneYet.body")}
        action={
          <GenerateReportDialog>
            <Button type="button" className="h-9 pointer-coarse:h-11">
              <Plus aria-hidden />
              {t("list.generate")}
            </Button>
          </GenerateReportDialog>
        }
      />
    </div>
  );
}
