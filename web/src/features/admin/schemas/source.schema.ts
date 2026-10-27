import { z } from "zod";

import {
  BLUESKY_MODES,
  BLUESKY_SORTS,
  DEVTO_TOPS,
  HN_LISTS,
  LOBSTERS_LISTS,
  MASTODON_MODES,
  MAX_GITHUB_DAYS,
  MAX_GITHUB_STARS,
  MAX_INTERVAL,
  MAX_PH_DAYS,
  MIN_INTERVAL,
  PH_ORDERS,
  REDDIT_SORTS,
  REDDIT_TIMES,
  secretMode,
  SOURCE_KINDS,
  SOURCE_MAX_LIMIT,
  TN_STRATEGIES,
} from "../lib/source-kinds";

const wholeNumber = /^\d+$/;

function inRange(value: string, min: number, max: number): boolean {
  const n = Number(value);
  return wholeNumber.test(value) && n >= min && n <= max;
}

/**
 * One flat form for every kind (easier for React Hook Form); only the fields
 * of the chosen kind are validated and sent. Messages are i18n keys under
 * common:fieldErrors.
 */
export const sourceFormSchema = z
  .object({
    kind: z.enum(SOURCE_KINDS),
    name: z.string().trim().min(1, { error: "required" }).max(120, { error: "max" }),
    interval: z.string().trim(),
    limit: z.string().trim(),
    hnList: z.enum(HN_LISTS),
    tnStrategy: z.enum(TN_STRATEGIES),
    subreddit: z.string().trim().max(100, { error: "max" }),
    redditSort: z.enum(REDDIT_SORTS),
    redditTime: z.enum(REDDIT_TIMES),
    channelId: z.string().trim().max(100, { error: "max" }),
    url: z.string().trim().max(2000, { error: "max" }),
    lobstersList: z.enum(LOBSTERS_LISTS),
    lobstersTag: z.string().trim().max(100, { error: "max" }),
    devtoTag: z.string().trim().max(100, { error: "max" }),
    devtoTop: z.enum(DEVTO_TOPS),
    mastodonInstance: z.string().trim().max(255, { error: "max" }),
    /** Form-only: which of hashtag/account is sent. */
    mastodonMode: z.enum(MASTODON_MODES),
    mastodonHashtag: z.string().trim().max(100, { error: "max" }),
    mastodonAccount: z.string().trim().max(255, { error: "max" }),
    bskyMode: z.enum(BLUESKY_MODES),
    bskyActor: z.string().trim().max(255, { error: "max" }),
    bskyFeed: z.string().trim().max(2000, { error: "max" }),
    bskyQuery: z.string().trim().max(300, { error: "max" }),
    bskySort: z.enum(BLUESKY_SORTS),
    bskyIdentifier: z.string().trim().max(255, { error: "max" }),
    ghQuery: z.string().trim().max(256, { error: "max" }),
    ghLanguage: z.string().trim().max(100, { error: "max" }),
    ghTopic: z.string().trim().max(100, { error: "max" }),
    ghMinStars: z.string().trim(),
    ghCreatedWithin: z.string().trim(),
    phOrder: z.enum(PH_ORDERS),
    phTopic: z.string().trim().max(100, { error: "max" }),
    phPostedWithin: z.string().trim(),
    /** Write-only credential; blank keeps the saved one (edit). */
    secret: z.string().max(4096, { error: "max" }),
    /** Remove the saved credential (optional-secret kinds only). */
    clearSecret: z.boolean(),
    /** Form-only: the source being edited already has a credential. */
    hasSecret: z.boolean(),
  })
  .superRefine((v, ctx) => {
    const fail = (path: string, message: string) =>
      ctx.addIssue({ code: "custom", path: [path], message });

    const interval = Number(v.interval);
    if (!wholeNumber.test(v.interval) || interval < MIN_INTERVAL || interval > MAX_INTERVAL)
      fail("interval", "invalid");

    const limit = Number(v.limit);
    if (!wholeNumber.test(v.limit) || limit < 1 || limit > SOURCE_MAX_LIMIT[v.kind])
      fail("limit", "invalid");

    if (v.kind === "reddit" && !v.subreddit) fail("subreddit", "required");
    if (v.kind === "youtube" && !v.channelId) fail("channelId", "required");
    if (v.kind === "rss") {
      if (!v.url) fail("url", "required");
      else if (!isHttpUrl(v.url)) fail("url", "url");
    }
    if (v.kind === "mastodon") {
      if (!v.mastodonInstance) fail("mastodonInstance", "required");
      if (v.mastodonMode === "hashtag" && !v.mastodonHashtag) fail("mastodonHashtag", "required");
      if (v.mastodonMode === "account" && !v.mastodonAccount) fail("mastodonAccount", "required");
    }
    if (v.kind === "bluesky") {
      if (v.bskyMode === "author" && !v.bskyActor) fail("bskyActor", "required");
      if (v.bskyMode === "feed" && !v.bskyFeed) fail("bskyFeed", "required");
      if (v.bskyMode === "search") {
        if (!v.bskyQuery) fail("bskyQuery", "required");
        if (!v.bskyIdentifier) fail("bskyIdentifier", "required");
      }
    }
    if (v.kind === "github") {
      if (!inRange(v.ghMinStars, 1, MAX_GITHUB_STARS)) fail("ghMinStars", "invalid");
      if (!inRange(v.ghCreatedWithin, 1, MAX_GITHUB_DAYS)) fail("ghCreatedWithin", "invalid");
    }
    if (v.kind === "producthunt" && !inRange(v.phPostedWithin, 1, MAX_PH_DAYS))
      fail("phPostedWithin", "invalid");

    // A required credential is mandatory when there is none saved (or it is being removed).
    if (secretMode(v.kind, v) === "required" && (!v.hasSecret || v.clearSecret) && !v.secret.trim())
      fail("secret", "required");
  });

export type SourceFormValues = z.infer<typeof sourceFormSchema>;

export function isHttpUrl(value: string): boolean {
  try {
    const u = new URL(value);
    return u.protocol === "http:" || u.protocol === "https:";
  } catch {
    return false;
  }
}
