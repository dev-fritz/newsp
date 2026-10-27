import {
  BLUESKY_MODES,
  BLUESKY_SORTS,
  DEFAULT_INTERVAL,
  DEFAULT_LIMIT,
  DEVTO_TOPS,
  HN_LISTS,
  LOBSTERS_LISTS,
  MASTODON_MODES,
  PH_ORDERS,
  REDDIT_SORTS,
  REDDIT_TIMES,
  secretMode,
  TN_STRATEGIES,
} from "./source-kinds";
import type { SourceFormValues } from "../schemas/source.schema";
import type { Source, SourceKind } from "../types";

export function emptySourceValues(kind: SourceKind = "rss"): SourceFormValues {
  return {
    kind,
    name: "",
    interval: DEFAULT_INTERVAL,
    limit: String(DEFAULT_LIMIT[kind]),
    hnList: "top",
    tnStrategy: "relevant",
    subreddit: "",
    redditSort: "top",
    redditTime: "day",
    channelId: "",
    url: "",
    lobstersList: "hottest",
    lobstersTag: "",
    devtoTag: "",
    devtoTop: "0",
    mastodonInstance: "",
    mastodonMode: "hashtag",
    mastodonHashtag: "",
    mastodonAccount: "",
    bskyMode: "author",
    bskyActor: "",
    bskyFeed: "",
    bskyQuery: "",
    bskySort: "top",
    bskyIdentifier: "",
    ghQuery: "",
    ghLanguage: "",
    ghTopic: "",
    ghMinStars: "10",
    ghCreatedWithin: "7",
    phOrder: "ranking",
    phTopic: "",
    phPostedWithin: "1",
    secret: "",
    clearSecret: false,
    hasSecret: false,
  };
}

function pick<T extends string>(options: readonly T[], value: unknown, fallback: T): T {
  return options.find((o) => o === value) ?? fallback;
}

function text(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function whole(value: unknown, fallback: string): string {
  return typeof value === "number" ? String(value) : fallback;
}

/** Form values of an existing source (edit mode). */
export function sourceToValues(source: Source): SourceFormValues {
  const base = emptySourceValues(source.kind);
  const c = source.config;
  return {
    ...base,
    name: source.name,
    interval: String(source.interval_minutes),
    limit: typeof c.limit === "number" ? String(c.limit) : base.limit,
    hnList: pick(HN_LISTS, c.list, base.hnList),
    tnStrategy: pick(TN_STRATEGIES, c.strategy, base.tnStrategy),
    subreddit: text(c.subreddit),
    redditSort: pick(REDDIT_SORTS, c.sort, base.redditSort),
    redditTime: pick(REDDIT_TIMES, c.time, base.redditTime),
    channelId: text(c.channel_id),
    url: text(c.url),
    lobstersList: pick(LOBSTERS_LISTS, c.list, base.lobstersList),
    lobstersTag: text(c.tag),
    devtoTag: text(c.tag),
    devtoTop: pick(DEVTO_TOPS, typeof c.top === "number" ? String(c.top) : "", base.devtoTop),
    mastodonInstance: text(c.instance),
    mastodonMode: pick(MASTODON_MODES, text(c.account) ? "account" : "hashtag", base.mastodonMode),
    mastodonHashtag: text(c.hashtag),
    mastodonAccount: text(c.account),
    bskyMode: pick(BLUESKY_MODES, c.mode, base.bskyMode),
    bskyActor: text(c.actor),
    bskyFeed: text(c.feed),
    bskyQuery: text(c.query),
    bskySort: pick(BLUESKY_SORTS, c.sort, base.bskySort),
    bskyIdentifier: text(c.identifier),
    ghQuery: text(c.query),
    ghLanguage: text(c.language),
    ghTopic: text(c.topic),
    ghMinStars: whole(c.min_stars, base.ghMinStars),
    ghCreatedWithin: whole(c.created_within_days, base.ghCreatedWithin),
    phOrder: pick(PH_ORDERS, c.order, base.phOrder),
    phTopic: text(c.topic),
    phPostedWithin: whole(c.posted_within_days, base.phPostedWithin),
    secret: "",
    clearSecret: false,
    hasSecret: source.has_secret === true,
  };
}

/** Optional text fields are left out of the config when blank. */
function optional(key: string, value: string): Record<string, string> {
  return value ? { [key]: value } : {};
}

function blueskyConfig(v: SourceFormValues, limit: number): Record<string, unknown> {
  switch (v.bskyMode) {
    case "author":
      return { mode: "author", actor: v.bskyActor, limit };
    case "feed":
      return { mode: "feed", feed: v.bskyFeed, limit };
    case "search":
      return {
        mode: "search",
        query: v.bskyQuery,
        sort: v.bskySort,
        identifier: v.bskyIdentifier,
        limit,
      };
  }
}

/** The connector config of the chosen kind, as the API expects it. */
export function valuesToConfig(v: SourceFormValues): Record<string, unknown> {
  const limit = Number(v.limit);
  switch (v.kind) {
    case "hackernews":
      return { list: v.hnList, limit };
    case "tabnews":
      return { strategy: v.tnStrategy, limit };
    case "reddit":
      return { subreddit: v.subreddit, sort: v.redditSort, time: v.redditTime, limit };
    case "youtube":
      return { channel_id: v.channelId, limit };
    case "rss":
      return { url: v.url, limit };
    case "lobsters":
      return { list: v.lobstersList, ...optional("tag", v.lobstersTag), limit };
    case "devto":
      return { top: Number(v.devtoTop), ...optional("tag", v.devtoTag), limit };
    case "mastodon":
      return {
        instance: v.mastodonInstance,
        ...(v.mastodonMode === "hashtag"
          ? { hashtag: v.mastodonHashtag }
          : { account: v.mastodonAccount }),
        limit,
      };
    case "bluesky":
      return blueskyConfig(v, limit);
    case "github":
      return {
        ...optional("query", v.ghQuery),
        ...optional("language", v.ghLanguage),
        ...optional("topic", v.ghTopic),
        min_stars: Number(v.ghMinStars),
        created_within_days: Number(v.ghCreatedWithin),
        limit,
      };
    case "producthunt":
      return {
        order: v.phOrder,
        ...optional("topic", v.phTopic),
        posted_within_days: Number(v.phPostedWithin),
        limit,
      };
  }
}

/** API field names (config.<field>, name, ...) to the form's field names, shared by every kind. */
const COMMON_FIELD_MAP: Record<string, keyof SourceFormValues> = {
  name: "name",
  interval_minutes: "interval",
  secret: "secret",
  "config.limit": "limit",
};

/** Config fields by kind: the same API name can mean different form fields per kind. */
const KIND_FIELD_MAP: Record<SourceKind, Record<string, keyof SourceFormValues>> = {
  hackernews: { "config.list": "hnList" },
  tabnews: { "config.strategy": "tnStrategy" },
  reddit: {
    "config.subreddit": "subreddit",
    "config.sort": "redditSort",
    "config.time": "redditTime",
  },
  youtube: { "config.channel_id": "channelId" },
  rss: { "config.url": "url" },
  lobsters: { "config.list": "lobstersList", "config.tag": "lobstersTag" },
  devto: { "config.tag": "devtoTag", "config.top": "devtoTop" },
  mastodon: {
    "config.instance": "mastodonInstance",
    "config.hashtag": "mastodonHashtag",
    "config.account": "mastodonAccount",
  },
  bluesky: {
    "config.mode": "bskyMode",
    "config.actor": "bskyActor",
    "config.feed": "bskyFeed",
    "config.query": "bskyQuery",
    "config.sort": "bskySort",
    "config.identifier": "bskyIdentifier",
  },
  github: {
    "config.query": "ghQuery",
    "config.language": "ghLanguage",
    "config.topic": "ghTopic",
    "config.min_stars": "ghMinStars",
    "config.created_within_days": "ghCreatedWithin",
  },
  producthunt: {
    "config.order": "phOrder",
    "config.topic": "phTopic",
    "config.posted_within_days": "phPostedWithin",
  },
};

/** Maps the API's validation fields of a source of the given kind to form fields. */
export function mapSourceFieldErrors(
  fields: Record<string, string> | undefined,
  kind: SourceKind,
): { field: keyof SourceFormValues; message: string }[] {
  const table = { ...COMMON_FIELD_MAP, ...KIND_FIELD_MAP[kind] };
  const out: { field: keyof SourceFormValues; message: string }[] = [];
  for (const [key, message] of Object.entries(fields ?? {})) {
    const field = table[key];
    if (field) out.push({ field, message });
  }
  return out;
}

/**
 * The credential part of the request body: "" removes the saved one, a value
 * replaces (or sets) it and omitting keeps it. Kinds/modes without a secret send nothing.
 */
export function secretPayload(v: SourceFormValues): { secret?: string } {
  const mode = secretMode(v.kind, v);
  if (mode === "none") return {};
  if (v.clearSecret) return { secret: "" };
  const secret = v.secret.trim();
  return secret ? { secret } : {};
}
