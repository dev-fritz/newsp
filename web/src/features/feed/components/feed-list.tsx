import { useTranslation } from "react-i18next";

import { EmptyState, ErrorState } from "@/components/common/states";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

import { useItemsInfinite } from "../api/feed-queries";
import type { FeedFilters } from "../schemas/feed-filters.schema";
import { FeedItemRow } from "./feed-item-row";

type Props = {
  filters: FeedFilters;
  hasFilters: boolean;
  onClear: () => void;
};

/** The item list with load more, plus its loading, empty and error states. */
export function FeedList({ filters, hasFilters, onClear }: Props) {
  const { t, i18n } = useTranslation(["feed", "common"]);
  const query = useItemsInfinite(filters);

  if (query.isPending) {
    return (
      <div className="flex flex-col gap-5" aria-busy="true" aria-label={t("common:states.loading")}>
        {Array.from({ length: 5 }, (_, i) => (
          <Skeleton key={i} className="h-28 w-full rounded-xl" />
        ))}
      </div>
    );
  }
  if (query.isError) return <ErrorState error={query.error} onRetry={() => void query.refetch()} />;

  const items = query.data.pages.flatMap((page) => page.data);
  const total = query.data.pages[0]?.meta.total ?? items.length;

  if (items.length === 0) {
    return hasFilters ? (
      <EmptyState
        title={t("feed:empty.filteredTitle")}
        body={t("feed:empty.filteredBody")}
        action={
          <Button variant="outline" onClick={onClear}>
            {t("feed:empty.clear")}
          </Button>
        }
      />
    ) : (
      <EmptyState title={t("feed:empty.title")} body={t("feed:empty.body")} />
    );
  }

  return (
    <section aria-label={t("feed:list.label")} className="flex flex-col">
      <p className="font-mono text-xs text-subtle-foreground" aria-live="polite">
        {t("feed:list.count", {
          count: total,
          value: new Intl.NumberFormat(i18n.language).format(total),
        })}
      </p>
      {items.map((item, index) => (
        <FeedItemRow key={item.id ?? index} item={item} />
      ))}
      {query.hasNextPage && (
        <div className="flex justify-center pt-6">
          <Button
            variant="outline"
            disabled={query.isFetchingNextPage}
            onClick={() => void query.fetchNextPage()}
            className="pointer-coarse:h-11"
          >
            {t("common:actions.loadMore")}
          </Button>
        </div>
      )}
      {query.isFetchNextPageError && (
        <p role="alert" className="pt-3 text-center text-sm text-negative">
          {t("feed:list.loadMoreError")}
        </p>
      )}
    </section>
  );
}
