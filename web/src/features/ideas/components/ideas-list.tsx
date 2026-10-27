import { Lightbulb } from "lucide-react";
import { useTranslation } from "react-i18next";

import { EmptyState, ErrorState } from "@/components/common/states";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

import { useIdeas } from "../api/ideas-queries";
import { useIdeaGeneration } from "../hooks/use-idea-generation";
import type { IdeaStatusFilter } from "../schemas/idea.schema";
import { GenerationProgressCard } from "./generation-progress-card";
import { IdeaCard } from "./idea-card";

const GRID = "grid grid-cols-[repeat(auto-fit,minmax(min(320px,100%),1fr))] gap-[18px]";

type Props = {
  status: IdeaStatusFilter;
  query: string;
};

/** Responsive grid of ideas with the generation placeholder, load more and empty state. */
export function IdeasList({ status, query }: Props) {
  const { t } = useTranslation(["ideas", "common"]);
  const ideas = useIdeas(status, query);
  const { run, running, queued, openDialog } = useIdeaGeneration();
  const showProgress = running && run !== null && (status === "active" || status === "new");

  if (ideas.isPending) {
    return (
      <div className={GRID} aria-busy="true" aria-label={t("common:states.loading")}>
        {Array.from({ length: 3 }, (_, i) => (
          <Skeleton key={i} className="h-[280px] rounded-xl" />
        ))}
      </div>
    );
  }
  if (ideas.isError) return <ErrorState error={ideas.error} onRetry={() => void ideas.refetch()} />;

  const items = ideas.data.pages.flatMap((page) => page.data);
  if (items.length === 0 && !showProgress) {
    return (
      <EmptyState
        title={query ? t("ideas:empty.searchTitle") : t(`ideas:empty.${status}Title`)}
        body={query ? t("ideas:empty.searchBody", { query }) : t(`ideas:empty.${status}Body`)}
        action={
          query ? undefined : (
            <Button type="button" aria-haspopup="dialog" onClick={() => openDialog()}>
              <Lightbulb aria-hidden />
              {t("ideas:empty.action")}
            </Button>
          )
        }
      />
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className={GRID}>
        {showProgress && (
          <GenerationProgressCard count={run.count} queued={queued} onOpen={() => openDialog()} />
        )}
        {items.map((idea) => (
          <IdeaCard key={idea.id} idea={idea} />
        ))}
      </div>
      {ideas.hasNextPage && (
        <Button
          variant="outline"
          className="self-center"
          disabled={ideas.isFetchingNextPage}
          onClick={() => void ideas.fetchNextPage()}
        >
          {ideas.isFetchingNextPage ? t("common:states.loading") : t("common:actions.loadMore")}
        </Button>
      )}
    </div>
  );
}
