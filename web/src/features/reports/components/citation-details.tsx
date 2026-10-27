import { useTranslation } from "react-i18next";

import { SheetClose, SheetTitle } from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

import { formatRelative, safeHttpUrl } from "../format";
import { useEngagementLabel } from "../hooks/use-engagement-label";
import type { Citation } from "../types";
import { SourceTag } from "./source-tag";

type CitationDetailsProps = {
  citation: Citation;
  /** "sheet" renders for the bottom sheet (title is the dialog title, full-width actions). */
  variant: "popover" | "sheet";
};

/** The cited item: source, date, engagement, title and a link to the original. */
export function CitationDetails({ citation, variant }: CitationDetailsProps) {
  const { t, i18n } = useTranslation("reports");
  const engagement = useEngagementLabel()(citation);
  const url = safeHttpUrl(citation.url);
  const sheet = variant === "sheet";
  const meta = [
    citation.source_name,
    formatRelative(citation.published_at, i18n.language),
    engagement,
  ]
    .filter(Boolean)
    .join(" · ");
  const title = citation.title ?? "";

  return (
    <div className={cn("flex min-w-0 flex-col gap-2.5 font-sans", sheet && "gap-3.5")}>
      <div className="flex min-w-0 items-center gap-2">
        {sheet && (
          <span className="inline-flex h-[18px] min-w-5 items-center justify-center rounded-[5px] bg-signal-soft px-1.5 font-mono text-[11px] font-semibold text-signal-ink">
            {citation.ref}
          </span>
        )}
        <SourceTag kind={citation.source_kind} />
        <span className="min-w-0 truncate text-[12.5px] text-subtle-foreground">{meta}</span>
      </div>
      {sheet ? (
        <SheetTitle className="font-sans text-[17px] leading-snug font-semibold">
          {title}
        </SheetTitle>
      ) : (
        <p className="text-sm leading-snug font-semibold text-foreground">{title}</p>
      )}
      {citation.author && (
        <p className="text-[12.5px] text-muted-foreground">
          {t("citation.by", { author: citation.author })}
        </p>
      )}
      {citation.excerpt && (
        <p className="line-clamp-4 text-[13px] leading-[1.45] text-muted-foreground">
          {t("citation.excerpt", { text: citation.excerpt })}
        </p>
      )}
      {sheet ? (
        <div className="grid grid-cols-2 gap-2 pt-1">
          <SheetClose asChild>
            <Button variant="secondary" className="h-12 text-sm">
              {t("citation.close")}
            </Button>
          </SheetClose>
          {url && (
            <Button asChild className="h-12 text-sm">
              <a href={url} target="_blank" rel="noopener noreferrer">
                {t("citation.openOriginal")}
              </a>
            </Button>
          )}
        </div>
      ) : (
        url && (
          <a
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            className="w-fit rounded-sm text-[13px] font-medium text-signal hover:underline focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
          >
            {t("citation.openOriginal")}
          </a>
        )
      )}
    </div>
  );
}
