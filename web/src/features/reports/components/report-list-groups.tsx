import { useTranslation } from "react-i18next";

import { groupByWeek } from "../format";
import type { ReportSummary } from "../types";
import { ReportListItem } from "./report-list-item";

/** Finished and failed reports grouped by week ("Esta semana", "Semana passada", then months). */
export function ReportListGroups({
  reports,
  selectedId,
}: {
  reports: ReportSummary[];
  selectedId: string | undefined;
}) {
  const { t, i18n } = useTranslation("reports");
  const groups = groupByWeek(reports, (r) => r.window_end ?? r.created_at, i18n.language);
  return (
    <>
      {groups.map((group) => (
        <section key={group.key} className="flex flex-col gap-0.5">
          <h2 className="kicker px-2.5 pt-4 pb-1.5">
            {group.label.kind === "month" ? group.label.text : t(`list.groups.${group.label.kind}`)}
          </h2>
          {group.items.map((report) => (
            <ReportListItem key={report.id} report={report} selected={report.id === selectedId} />
          ))}
        </section>
      ))}
    </>
  );
}
