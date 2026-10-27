import { type InfiniteData, useInfiniteQuery, useQuery } from "@tanstack/react-query";

import type { ApiError } from "@/lib/api-error";

import type { FeedFilters, FeedPeriod } from "../schemas/feed-filters.schema";
import type { FeedItem, FeedSource } from "../types";
import { fetchItems, fetchSources, type ItemsPage } from "./feed-api";

export const FEED_PAGE_SIZE = 20;
const TOP_LIMIT = 8;

export const feedKeys = {
  all: ["items"] as const,
  lists: () => [...feedKeys.all, "list"] as const,
  list: (filters: FeedFilters) => [...feedKeys.lists(), filters] as const,
  top: () => [...feedKeys.all, "top"] as const,
  search: (q: string, period: FeedPeriod) => [...feedKeys.all, "search", { q, period }] as const,
  sources: () => ["sources", "summary"] as const,
};

/** Items matching the filters, loaded page by page. */
export function useItemsInfinite(filters: FeedFilters) {
  return useInfiniteQuery<
    ItemsPage,
    ApiError,
    InfiniteData<ItemsPage, number>,
    ReturnType<typeof feedKeys.list>,
    number
  >({
    queryKey: feedKeys.list(filters),
    initialPageParam: 0,
    queryFn: ({ pageParam }) =>
      fetchItems({
        q: filters.q,
        sourceId: filters.source,
        period: filters.period,
        sort: filters.sort,
        limit: FEED_PAGE_SIZE,
        offset: pageParam,
      }),
    getNextPageParam: (last) => {
      const next = last.meta.offset + last.data.length;
      return last.data.length > 0 && next < last.meta.total ? next : undefined;
    },
  });
}

/** The most engaging items of the last 24 hours, for the home screen. */
export function useTopItems() {
  return useQuery<FeedItem[], ApiError>({
    queryKey: feedKeys.top(),
    queryFn: async () =>
      (await fetchItems({ period: "24h", sort: "engagement", limit: TOP_LIMIT, offset: 0 })).data,
  });
}

const SEARCH_LIMIT = 8;

/** The most engaging items of a period matching a query, for item pickers. */
export function useItemSearch(q: string, period: FeedPeriod) {
  return useQuery<FeedItem[], ApiError>({
    queryKey: feedKeys.search(q, period),
    queryFn: async () =>
      (await fetchItems({ q, period, sort: "engagement", limit: SEARCH_LIMIT, offset: 0 })).data,
    placeholderData: (previous) => previous,
  });
}

/** Sources for the feed's source filter. */
export function useFeedSources() {
  return useQuery<FeedSource[], ApiError>({
    queryKey: feedKeys.sources(),
    queryFn: fetchSources,
    staleTime: 60_000,
  });
}
