import { Plus } from "lucide-react";
import { useTranslation } from "react-i18next";

import { EmptyState, ErrorState, ListSkeleton } from "@/components/common/states";
import { Button } from "@/components/ui/button";

import { useReportsInfinite } from "../api/reports-queries";
import { useKindFilter } from "../hooks/use-kind-filter";
import { isActive } from "../types";
import { GenerateReportDialog } from "./generate-report-dialog";
import { ReportFilter } from "./report-filter";
import { ReportListGroups } from "./report-list-groups";
import { ReportProgressCard } from "./report-progress-card";

/** Left column / list page: title, generate action, filter and the grouped reports. */
export function ReportList({ selectedId }: { selectedId: string | undefined }) {
  const { t } = useTranslation("reports");
  const [kind, setKind] = useKindFilter();
  const query = useReportsInfinite(kind);
  const reports = query.data?.pages.flatMap((page) => page.data) ?? [];
  const active = reports.filter(isActive);
  const finished = reports.filter((report) => !isActive(report));

  const generate = (
    <GenerateReportDialog>
      <Button type="button" className="h-8 pointer-coarse:h-11">
        <Plus aria-hidden />
        {t("list.generate")}
      </Button>
    </GenerateReportDialog>
  );

  return (
    <section aria-label={t("list.label")} className="flex min-w-0 flex-col">
      <div className="flex flex-col gap-3.5 border-b border-border pb-4">
        <div className="flex items-center justify-between gap-2">
          <h1 className="font-serif text-[28px] leading-[1.1] font-medium">{t("title")}</h1>
          {generate}
        </div>
        <ReportFilter value={kind} onChange={setKind} />
      </div>

      <div className="flex flex-col gap-0.5 pt-2 pb-6">
        {query.isPending && <ListSkeleton rows={5} />}
        {query.isError && <ErrorState error={query.error} onRetry={() => void query.refetch()} />}

        {query.isSuccess && reports.length === 0 && (
          <EmptyState
            title={t(kind ? "list.emptyFiltered.title" : "list.empty.title")}
            body={t(kind ? "list.emptyFiltered.body" : "list.empty.body")}
            action={
              kind ? undefined : (
                <GenerateReportDialog>
                  <Button type="button" className="h-9 pointer-coarse:h-11">
                    <Plus aria-hidden />
                    {t("list.generate")}
                  </Button>
                </GenerateReportDialog>
              )
            }
          />
        )}

        {active.map((report) => (
          <div key={report.id} className="my-1.5">
            <ReportProgressCard report={report} />
          </div>
        ))}

        <ReportListGroups reports={finished} selectedId={selectedId} />

        {query.hasNextPage && (
          <Button
            type="button"
            variant="outline"
            className="mt-4 h-9 pointer-coarse:h-11"
            disabled={query.isFetchingNextPage}
            onClick={() => void query.fetchNextPage()}
          >
            {t("common:actions.loadMore")}
          </Button>
        )}
      </div>
    </section>
  );
}
