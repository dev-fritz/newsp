import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router";

import {
  DEFAULT_PERIOD,
  DEFAULT_SORT,
  feedFiltersSchema,
  type FeedFilters,
} from "../schemas/feed-filters.schema";

const FILTER_KEYS = ["q", "source", "period", "sort"] as const;

/** The feed filters live in the URL (`?q=&source=&period=&sort=`), validated with Zod. */
export function useFeedFilters(): {
  filters: FeedFilters;
  /** Whether anything narrows the list (so an empty result may be caused by the filters). */
  hasFilters: boolean;
  setFilters: (patch: Partial<FeedFilters>) => void;
  clearFilters: () => void;
} {
  const [params, setParams] = useSearchParams();
  const filters = useMemo(
    () =>
      feedFiltersSchema.parse({
        q: params.get("q") ?? "",
        source: params.get("source") ?? undefined,
        period: params.get("period") ?? undefined,
        sort: params.get("sort") ?? undefined,
      }),
    [params],
  );

  const setFilters = useCallback(
    (patch: Partial<FeedFilters>) => {
      setParams(
        (prev) => {
          const next = feedFiltersSchema.parse({
            q: prev.get("q") ?? "",
            source: prev.get("source") ?? undefined,
            period: prev.get("period") ?? undefined,
            sort: prev.get("sort") ?? undefined,
            ...patch,
          });
          const copy = new URLSearchParams(prev);
          for (const key of FILTER_KEYS) copy.delete(key);
          if (next.q) copy.set("q", next.q);
          if (next.source) copy.set("source", next.source);
          if (next.period !== DEFAULT_PERIOD) copy.set("period", next.period);
          if (next.sort !== DEFAULT_SORT) copy.set("sort", next.sort);
          return copy;
        },
        { replace: true },
      );
    },
    [setParams],
  );

  const clearFilters = useCallback(() => {
    setParams(
      (prev) => {
        const copy = new URLSearchParams(prev);
        for (const key of FILTER_KEYS) copy.delete(key);
        // "Clear" shows everything collected, not just the default period.
        copy.set("period", "all");
        return copy;
      },
      { replace: true },
    );
  }, [setParams]);

  const hasFilters = filters.q !== "" || filters.source !== undefined || filters.period !== "all";
  return { filters, hasFilters, setFilters, clearFilters };
}
