import { getData, getPage, type PageMeta } from "@/lib/http";

import type { FeedPeriod, FeedSort } from "../schemas/feed-filters.schema";
import type { FeedItem, FeedSource } from "../types";

export type ItemsQuery = {
  q?: string;
  sourceId?: string;
  period?: FeedPeriod;
  sort: FeedSort;
  limit: number;
  offset: number;
};

export type ItemsPage = { data: FeedItem[]; meta: PageMeta };

const HOUR_MS = 3_600_000;
const PERIOD_HOURS: Record<Exclude<FeedPeriod, "all">, number> = {
  "24h": 24,
  "7d": 24 * 7,
  "30d": 24 * 30,
};

/** RFC 3339 lower bound of a period, relative to now; undefined for "all". */
export function sinceOf(period: FeedPeriod | undefined, now = Date.now()): string | undefined {
  if (!period || period === "all") return undefined;
  return new Date(now - PERIOD_HOURS[period] * HOUR_MS).toISOString();
}

export function fetchItems(query: ItemsQuery): Promise<ItemsPage> {
  return getPage<FeedItem>("/items", {
    q: query.q || undefined,
    source_id: query.sourceId,
    since: sinceOf(query.period),
    sort: query.sort,
    limit: query.limit,
    offset: query.offset,
  });
}

export function fetchSources(): Promise<FeedSource[]> {
  return getData<FeedSource[]>("/sources");
}
