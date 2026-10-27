import { useTranslation } from "react-i18next";

import type { Source } from "../types";

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function hostAndPath(url: string): string {
  try {
    const u = new URL(url);
    return `${u.host}${u.pathname === "/" ? "" : u.pathname}`;
  } catch {
    return url;
  }
}

/** One-line summary of a source's config (list and limit, subreddit, feed URL...). */
export function SourceDetail({ source }: { source: Source }) {
  const { t } = useTranslation("admin");
  const c = source.config;
  const label = (group: string, value: unknown) =>
    t(`sources.options.${group}.${str(value)}`, { defaultValue: str(value) });
  const limit = typeof c.limit === "number" ? c.limit : undefined;
  let text: string;
  switch (source.kind) {
    case "hackernews":
      text = t("sources.detail.hackernews", { list: label("hnList", c.list), limit });
      break;
    case "tabnews":
      text = t("sources.detail.tabnews", { strategy: label("tnStrategy", c.strategy), limit });
      break;
    case "reddit":
      text = t("sources.detail.reddit", {
        subreddit: str(c.subreddit).replace(/^\/?r\//i, ""),
        sort: label("redditSort", c.sort),
        limit,
      });
      break;
    case "youtube":
      text = t("sources.detail.youtube", { channel: str(c.channel_id), limit });
      break;
    case "rss":
      text = hostAndPath(str(c.url));
      break;
    case "lobsters":
      text = c.tag
        ? t("sources.detail.lobstersTag", { tag: str(c.tag), limit })
        : t("sources.detail.lobsters", { list: label("lobstersList", c.list), limit });
      break;
    case "devto":
      text = t("sources.detail.devto", {
        tag: str(c.tag) || t("sources.detail.anyTag"),
        top: label("devtoTop", String(c.top ?? 0)),
        limit,
      });
      break;
    case "mastodon":
      text = t("sources.detail.mastodon", {
        target: c.account
          ? `@${str(c.account).replace(/^@/, "")}`
          : `#${str(c.hashtag).replace(/^#/, "")}`,
        instance: str(c.instance),
        limit,
      });
      break;
    case "bluesky":
      text = t("sources.detail.bluesky", {
        mode: label("bskyMode", c.mode),
        target: str(c.actor) || str(c.query) || str(c.feed),
        limit,
      });
      break;
    case "github":
      text = t("sources.detail.github", {
        query: [str(c.query), str(c.language), str(c.topic)].filter(Boolean).join(" ") || "*",
        stars: typeof c.min_stars === "number" ? c.min_stars : undefined,
        days: typeof c.created_within_days === "number" ? c.created_within_days : undefined,
        limit,
      });
      break;
    case "producthunt":
      text = t("sources.detail.producthunt", {
        order: label("phOrder", c.order),
        days: typeof c.posted_within_days === "number" ? c.posted_within_days : undefined,
        limit,
      });
      break;
  }
  return (
    <span
      className="block max-w-72 truncate font-mono text-[11.5px] text-subtle-foreground"
      title={text}
    >
      {text}
    </span>
  );
}
