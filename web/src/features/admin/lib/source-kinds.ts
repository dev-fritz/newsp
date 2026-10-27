import type { SourceKind } from "../types";

export const SOURCE_KINDS = [
  "hackernews",
  "tabnews",
  "reddit",
  "youtube",
  "rss",
  "lobsters",
  "devto",
  "mastodon",
  "bluesky",
  "github",
  "producthunt",
] as const;

/** Monochrome masthead tag shown next to each source. */
export const KIND_TAG: Record<SourceKind, string> = {
  hackernews: "HN",
  tabnews: "TN",
  reddit: "r/",
  youtube: "YT",
  rss: "RSS",
  lobsters: "LO",
  devto: "DEV",
  mastodon: "M",
  bluesky: "BS",
  github: "GH",
  producthunt: "PH",
};

/** Upper bound of "limit" per connector. */
export const SOURCE_MAX_LIMIT: Record<SourceKind, number> = {
  hackernews: 100,
  tabnews: 100,
  reddit: 100,
  youtube: 50,
  rss: 200,
  lobsters: 25,
  devto: 100,
  mastodon: 40,
  bluesky: 100,
  github: 100,
  producthunt: 50,
};

export const DEFAULT_LIMIT: Record<SourceKind, number> = {
  hackernews: 30,
  tabnews: 30,
  reddit: 30,
  youtube: 15,
  rss: 30,
  lobsters: 25,
  devto: 30,
  mastodon: 20,
  bluesky: 30,
  github: 30,
  producthunt: 20,
};

export const INTERVAL_PRESETS = ["15", "30", "60", "360"] as const;
export const MIN_INTERVAL = 5;
export const MAX_INTERVAL = 1440;
export const DEFAULT_INTERVAL = "30";

export const HN_LISTS = ["top", "best", "new"] as const;
export const TN_STRATEGIES = ["relevant", "new", "old"] as const;
export const REDDIT_SORTS = ["top", "hot", "new"] as const;
export const REDDIT_TIMES = ["hour", "day", "week"] as const;
export const LOBSTERS_LISTS = ["hottest", "newest"] as const;
/** dev.to "top" window in days as a string (the select value); "0" is newest. */
export const DEVTO_TOPS = ["0", "1", "7", "30"] as const;
export const MASTODON_MODES = ["hashtag", "account"] as const;
export const BLUESKY_MODES = ["author", "feed", "search"] as const;
export const BLUESKY_SORTS = ["top", "latest"] as const;
export const PH_ORDERS = ["ranking", "votes", "newest"] as const;

export const MAX_GITHUB_STARS = 1_000_000;
export const MAX_GITHUB_DAYS = 30;
export const MAX_PH_DAYS = 7;

export type SecretMode = "none" | "optional" | "required";

/** Whether the kind takes a per-source credential (see secretMode for mode-dependent kinds). */
export const SECRET_MODE: Record<SourceKind, SecretMode> = {
  hackernews: "none",
  tabnews: "none",
  reddit: "none",
  youtube: "none",
  rss: "none",
  lobsters: "none",
  devto: "none",
  mastodon: "none",
  bluesky: "required",
  github: "optional",
  producthunt: "required",
};

/** The credential rule of a source: Bluesky only needs one to search. */
export function secretMode(kind: SourceKind, values: { bskyMode: string }): SecretMode {
  if (kind === "bluesky") return values.bskyMode === "search" ? "required" : "none";
  return SECRET_MODE[kind];
}
