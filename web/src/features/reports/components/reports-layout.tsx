import { cn } from "@/lib/utils";

import { LatestReportPanel, useDefaultReportId } from "./latest-report-panel";
import { ReportList } from "./report-list";

/**
 * Two columns on wide screens (list + reader); on narrower ones either the list
 * (`/reports`) or the reader (`/reports/:id`). Without an id the reader shows the latest report.
 */
export function ReportsLayout({
  selectedId,
  children,
}: {
  /** The `:id` of the open report, if any. */
  selectedId: string | undefined;
  /** The reader for `selectedId` (the route's `<Outlet />`). */
  children: React.ReactNode;
}) {
  const defaultId = useDefaultReportId();
  return (
    <div className="grid grid-cols-1 gap-x-8 xl:grid-cols-[300px_minmax(0,1fr)]">
      <div className={cn("min-w-0", selectedId ? "hidden xl:block" : "block")}>
        <ReportList selectedId={selectedId ?? defaultId} />
      </div>
      <div
        className={cn(
          "min-w-0 rounded-2xl border-border bg-card xl:border",
          selectedId ? "block" : "hidden xl:block",
        )}
      >
        {selectedId ? children : <LatestReportPanel />}
      </div>
    </div>
  );
}
