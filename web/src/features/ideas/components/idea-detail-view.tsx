import { useState } from "react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";

import { EmptyState, ErrorState } from "@/components/common/states";
import { Button } from "@/components/ui/button";

import { useIdea, useIdeaVersion } from "../api/ideas-queries";
import { useIdeaDeepening } from "../hooks/use-idea-deepening";
import { DeepenDialog } from "./deepen-dialog";
import { EditIdeaSheet } from "./edit-idea-sheet";
import { IdeaArticle } from "./idea-article";
import { IdeaDedupCard } from "./idea-dedup-card";
import { IdeaDetailSkeleton } from "./idea-detail-skeleton";
import { IdeaGenerationCard } from "./idea-generation-card";
import { IdeaMergedBanner } from "./idea-merged-banner";
import { IdeaMobileActions } from "./idea-mobile-actions";
import { IdeaToolbar } from "./idea-toolbar";
import { IdeaVersionsCard } from "./idea-versions-card";

/** One idea: toolbar, merge banner, the article and the side rail, plus edit and deepen. */
export function IdeaDetailView({ id }: { id: string }) {
  const { t } = useTranslation("ideas");
  const idea = useIdea(id);
  const deepening = useIdeaDeepening(id);
  const [editOpen, setEditOpen] = useState(false);
  const [deepenOpen, setDeepenOpen] = useState(false);

  // Versions made from an earlier one know what is new: evidence the earlier lacked.
  const current = idea.data?.current;
  const previousVersion =
    current && current.version > 1 && current.origin !== "generated" && current.origin !== "manual"
      ? current.version - 1
      : undefined;
  const previous = useIdeaVersion(id, previousVersion);

  if (idea.isPending) return <IdeaDetailSkeleton />;
  if (idea.isError) {
    if (idea.error.status === 404) {
      return (
        <EmptyState
          title={t("detail.notFoundTitle")}
          body={t("detail.notFoundBody")}
          action={
            <Button asChild variant="outline">
              <Link to="/ideas">{t("detail.backToList")}</Link>
            </Button>
          }
        />
      );
    }
    return <ErrorState error={idea.error} onRetry={() => void idea.refetch()} />;
  }

  const data = idea.data;
  const previousIds = new Set(previous.data?.evidence.map((item) => item.item_id));
  const newEvidenceIds = new Set(
    previous.data
      ? data.current.evidence
          .map((item) => item.item_id)
          .filter((itemId) => !previousIds.has(itemId))
      : [],
  );
  const openEdit = () => setEditOpen(true);
  const openDeepen = () => setDeepenOpen(true);

  return (
    <div className="flex flex-col gap-[26px] pb-24 lg:pb-0">
      <IdeaToolbar
        idea={data}
        onEdit={openEdit}
        onDeepen={openDeepen}
        deepening={deepening.running}
      />

      {deepening.running && (
        <output
          aria-live="polite"
          className="flex items-center gap-2 rounded-lg border border-dashed border-border-strong px-4 py-3 text-sm"
        >
          <span
            aria-hidden
            className="inline-block size-[7px] shrink-0 animate-pulse rounded-full bg-signal motion-reduce:animate-none"
          />
          {t("deepen.running")}
        </output>
      )}

      <div className="flex flex-wrap items-start gap-x-14 gap-y-8">
        <div className="flex min-w-0 flex-[999_1_560px] flex-col gap-[30px]">
          {data.current.origin === "merged" && data.current.version > 1 && (
            <IdeaMergedBanner
              ideaId={data.id}
              version={data.current.version}
              newEvidence={newEvidenceIds.size}
            />
          )}
          <IdeaArticle idea={data} newEvidenceIds={newEvidenceIds} />
        </div>
        <aside className="flex min-w-0 flex-[1_1_280px] flex-col gap-[18px] lg:sticky lg:top-6">
          <IdeaVersionsCard idea={data} />
          <IdeaDedupCard idea={data} />
          <IdeaGenerationCard idea={data} />
        </aside>
      </div>

      <IdeaMobileActions onEdit={openEdit} onDeepen={openDeepen} deepening={deepening.running} />
      <EditIdeaSheet idea={data} open={editOpen} onOpenChange={setEditOpen} />
      <DeepenDialog
        open={deepenOpen}
        onOpenChange={setDeepenOpen}
        deepening={deepening}
        currentVersion={data.current_version}
      />
    </div>
  );
}
