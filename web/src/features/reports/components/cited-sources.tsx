import { useTranslation } from "react-i18next";

import { safeHttpUrl } from "../format";
import { useEngagementLabel } from "../hooks/use-engagement-label";
import type { Citation } from "../types";
import { SourceTag } from "./source-tag";

/** "Fontes citadas": every cited item, numbered by its `[n]` reference. */
export function CitedSources({ citations }: { citations: Citation[] }) {
  const { t } = useTranslation("reports");
  const engagementLabel = useEngagementLabel();
  if (citations.length === 0) return null;
  const sorted = citations.toSorted((a, b) => (a.ref ?? 0) - (b.ref ?? 0));

  return (
    <section aria-labelledby="report-sources" className="mt-3 flex flex-col">
      <div className="flex items-baseline justify-between gap-3 border-b-2 border-foreground pb-3">
        <h2 id="report-sources" className="kicker text-foreground">
          {t("sources.title")}
        </h2>
        <span className="text-[12.5px] text-subtle-foreground">
          {t("sources.count", { count: sorted.length })}
        </span>
      </div>
      <ol className="flex flex-col">
        {sorted.map((citation) => {
          const url = safeHttpUrl(citation.url);
          const title = citation.title || citation.url || "";
          const engagement = engagementLabel(citation);
          return (
            <li
              key={citation.item_id ?? citation.ref}
              className="flex flex-wrap items-baseline gap-x-3.5 gap-y-1 border-b border-border py-3 last:border-b-0"
            >
              <span className="w-[18px] shrink-0 font-mono text-xs text-subtle-foreground">
                {citation.ref}
              </span>
              <SourceTag kind={citation.source_kind} />
              <span className="min-w-0 flex-[1_1_220px] font-sans text-sm break-words text-foreground">
                {url ? (
                  <a
                    href={url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="rounded-sm hover:underline focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
                  >
                    {title}
                  </a>
                ) : (
                  title
                )}
              </span>
              {engagement && (
                <span className="font-mono text-xs text-subtle-foreground">{engagement}</span>
              )}
            </li>
          );
        })}
      </ol>
    </section>
  );
}
