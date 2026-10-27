import { Plus } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { EmptyState, ErrorState, ListSkeleton } from "@/components/common/states";
import { PageHeader } from "@/components/common/page-header";
import { Button } from "@/components/ui/button";
import { useCan } from "@/features/auth";

import { useSources } from "../api/sources-queries";
import { errorOf, FORBIDDEN } from "../lib/error-of";
import type { Source } from "../types";
import { DeleteSourceDialog } from "./delete-source-dialog";
import { SourceSheet } from "./source-sheet";
import { SourcesStats } from "./sources-stats";
import { SourcesTable } from "./sources-table";

/** /admin/sources: health of every source, plus create, edit, enable and delete. */
export function SourcesPage() {
  const { t } = useTranslation("admin");
  const allowed = useCan("sources:manage");
  const sources = useSources();
  const [sheetOpen, setSheetOpen] = useState(false);
  const [editing, setEditing] = useState<Source | undefined>();
  const [deleting, setDeleting] = useState<Source | null>(null);

  function openSheet(source?: Source) {
    setEditing(source);
    setSheetOpen(true);
  }

  const header = (
    <PageHeader
      kicker={t("kicker")}
      title={t("sources.title")}
      actions={
        allowed && (
          <Button onClick={() => openSheet()} className="h-9 pointer-coarse:h-11">
            <Plus aria-hidden />
            {t("sources.new")}
          </Button>
        )
      }
    />
  );

  if (!allowed) {
    return (
      <div className="flex flex-col gap-6">
        {header}
        <ErrorState error={FORBIDDEN} />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      {header}
      {sources.isPending && <ListSkeleton rows={5} />}
      {sources.isError && (
        <ErrorState error={errorOf(sources.error)} onRetry={() => void sources.refetch()} />
      )}
      {sources.data && sources.data.length === 0 && (
        <EmptyState
          title={t("sources.empty.title")}
          body={t("sources.empty.body")}
          action={
            <Button onClick={() => openSheet()} className="pointer-coarse:h-11">
              <Plus aria-hidden />
              {t("sources.new")}
            </Button>
          }
        />
      )}
      {sources.data && sources.data.length > 0 && (
        <>
          <SourcesStats sources={sources.data} />
          <SourcesTable sources={sources.data} onEdit={openSheet} onDelete={setDeleting} />
        </>
      )}
      <SourceSheet
        key={editing?.id ?? "new"}
        open={sheetOpen}
        onOpenChange={setSheetOpen}
        source={editing}
      />
      <DeleteSourceDialog source={deleting} onClose={() => setDeleting(null)} />
    </div>
  );
}
