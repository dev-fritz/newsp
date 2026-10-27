import { Link } from "react-router";
import { useTranslation } from "react-i18next";

import { EmptyState, ErrorState } from "@/components/common/states";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { IdeaCard, useIdeaGeneration, useRecentIdeas } from "@/features/ideas";

const IDEAS_LIMIT = 4;

/** The "recent ideas" column: up to four idea cards and a link to the full list. */
export function HomeIdeas() {
  const { t } = useTranslation(["home", "common"]);
  const query = useRecentIdeas(IDEAS_LIMIT);
  const { openDialog } = useIdeaGeneration();

  let body: React.ReactNode;
  if (query.isPending) {
    body = (
      <div className="flex flex-col gap-4" aria-busy="true" aria-label={t("common:states.loading")}>
        {Array.from({ length: 3 }, (_, i) => (
          <Skeleton key={i} className="h-[280px] rounded-xl" />
        ))}
      </div>
    );
  } else if (query.isError) {
    body = <ErrorState error={query.error} onRetry={() => void query.refetch()} />;
  } else if (query.data.length === 0) {
    body = (
      <EmptyState
        title={t("home:ideas.empty.title")}
        body={t("home:ideas.empty.body")}
        action={
          <Button
            type="button"
            className="h-10 pointer-coarse:h-11"
            aria-haspopup="dialog"
            onClick={() => openDialog()}
          >
            {t("home:ideas.empty.action")}
          </Button>
        }
      />
    );
  } else {
    body = (
      <ul className="m-0 flex list-none flex-col gap-4 p-0">
        {query.data.map((idea) => (
          <li key={idea.id}>
            <IdeaCard idea={idea} />
          </li>
        ))}
      </ul>
    );
  }

  return (
    <section aria-labelledby="home-ideas" className="flex min-w-0 flex-col gap-4">
      <div className="flex items-baseline justify-between gap-3">
        <h2 id="home-ideas" className="font-serif text-2xl font-medium">
          {t("home:ideas.title")}
        </h2>
        <Link
          to="/ideas"
          className="rounded-sm text-sm text-signal outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {t("home:ideas.seeAll")}
        </Link>
      </div>
      {body}
    </section>
  );
}
