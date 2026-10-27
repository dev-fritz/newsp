import { z } from "zod";

export const FEED_PERIODS = ["24h", "7d", "30d", "all"] as const;
export const FEED_SORTS = ["recent", "engagement"] as const;

export const DEFAULT_PERIOD = "7d";
export const DEFAULT_SORT = "recent";
export const MAX_QUERY_LENGTH = 200;

/** The feed filters as they live in the URL; anything invalid falls back to the default. */
export const feedFiltersSchema = z.object({
  q: z.string().trim().max(MAX_QUERY_LENGTH).catch(""),
  source: z.uuid().optional().catch(undefined),
  period: z.enum(FEED_PERIODS).catch(DEFAULT_PERIOD),
  sort: z.enum(FEED_SORTS).catch(DEFAULT_SORT),
});

export type FeedFilters = z.infer<typeof feedFiltersSchema>;
export type FeedPeriod = (typeof FEED_PERIODS)[number];
export type FeedSort = (typeof FEED_SORTS)[number];
