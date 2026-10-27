import { ExternalLink } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";

import { useItemFormat } from "../hooks/use-item-format";
import { safeHttpUrl, toPlainText } from "../lib/item-text";
import type { FeedItem } from "../types";
import { ItemLink } from "./item-link";
import { SourceTag } from "./source-tag";

/** One collected item in the feed. */
export function FeedItemRow({ item }: { item: FeedItem }) {
  const { t } = useTranslation("feed");
  const format = useItemFormat();
  const summary = toPlainText(item.summary);
  const engagement = format.engagement(item);
  const href = safeHttpUrl(item.url);
  const title = item.title?.trim() || t("item.untitled");

  return (
    <article className="flex gap-3 border-b border-border py-5 sm:gap-4">
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[13px] text-subtle-foreground">
          <SourceTag kind={item.source?.kind} name={item.source?.name} />
          <span className="min-w-0 truncate">
            {item.source?.name}
            {item.author ? ` · ${t("item.by", { author: item.author })}` : ""}
          </span>
          <span aria-hidden>·</span>
          <time dateTime={item.published_at} className="font-mono text-xs">
            {format.relative(item.published_at)}
          </time>
        </span>
        <h2 className="max-w-[80ch] font-serif text-[21px] leading-[1.25] font-medium text-balance [overflow-wrap:anywhere]">
          <ItemLink url={item.url} title={title} />
        </h2>
        {summary && (
          <p className="line-clamp-3 max-w-[80ch] text-[14.5px] leading-[1.55] [overflow-wrap:anywhere] text-muted-foreground">
            {summary}
          </p>
        )}
        {(engagement || item.cited) && (
          <span className="mt-0.5 flex flex-wrap items-center gap-2">
            {engagement && (
              <span className="font-mono text-xs text-subtle-foreground">{engagement}</span>
            )}
            {item.cited && (
              <Badge className="border-transparent bg-signal-soft text-signal-ink">
                {t("item.cited")}
              </Badge>
            )}
          </span>
        )}
      </div>
      {href && (
        <a
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          aria-label={t("item.openOriginal", { title })}
          className="flex size-9 shrink-0 items-center justify-center rounded-lg text-muted-foreground outline-none hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 pointer-coarse:size-11"
        >
          <ExternalLink className="size-4" aria-hidden />
        </a>
      )}
    </article>
  );
}
