import { useTranslation } from "react-i18next";

import { Table, TableBody, TableHead, TableHeader, TableRow } from "@/components/ui/table";

import type { Source } from "../types";
import { SourceRow } from "./source-row";

type SourcesTableProps = {
  sources: Source[];
  onEdit: (source: Source) => void;
  onDelete: (source: Source) => void;
};

export function SourcesTable({ sources, onEdit, onDelete }: SourcesTableProps) {
  const { t } = useTranslation("admin");
  return (
    <div className="overflow-hidden rounded-2xl border border-border bg-card">
      <Table className="min-w-[860px]">
        <TableHeader>
          <TableRow>
            <TableHead className="kicker">{t("sources.table.source")}</TableHead>
            <TableHead className="kicker">{t("sources.table.interval")}</TableHead>
            <TableHead className="kicker">{t("sources.table.status")}</TableHead>
            <TableHead className="kicker">{t("sources.table.lastRun")}</TableHead>
            <TableHead className="kicker text-right">{t("sources.table.items")}</TableHead>
            <TableHead className="kicker text-right">{t("sources.table.actions")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sources.map((s) => (
            <SourceRow key={s.id} source={s} onEdit={onEdit} onDelete={onDelete} />
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
