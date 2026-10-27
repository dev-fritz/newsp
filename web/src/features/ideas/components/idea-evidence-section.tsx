import { useTranslation } from "react-i18next";

import { evidenceDomId, isHttpUrl } from "../lib/idea-text";
import type { Evidence } from "../schemas/idea-response.schema";
import { CiteChip } from "./cite-chip";
import { EvidenceMeta } from "./evidence-meta";
import { IdeaBadge } from "./idea-badge";
import { SourceTag } from "./source-tag";

type Props = {
  evidence: Evidence[];
  /** Item ids that first appeared in the current version (marked "new in vN"). */
  newItemIds: ReadonlySet<string>;
  version: number;
};

/** The items that back the idea; "[E1]" citations in the text jump here. */
export function IdeaEvidenceSection({ evidence, newItemIds, version }: Props) {
  const { t } = useTranslation("ideas");
  return (
    <section id="evidence" aria-labelledby="evidence-title" className="flex scroll-mt-24 flex-col">
      <div className="flex items-baseline justify-between gap-3 border-b-2 border-foreground pb-3">
        <h2
          id="evidence-title"
          className="font-serif text-[26px] leading-tight font-semibold sm:text-[28px]"
        >
          {t("sections.evidence")}
        </h2>
        <span className="text-[13px] text-subtle-foreground">
          {t("evidence.collected", { count: evidence.length })}
        </span>
      </div>
      {evidence.length === 0 ? (
        <p className="py-4 text-sm text-muted-foreground">{t("evidence.none")}</p>
      ) : (
        <ol className="m-0 flex list-none flex-col p-0">
          {evidence.map((item) => (
            <li
              key={item.item_id || item.ref}
              id={evidenceDomId(item.ref)}
              className="-mx-2 flex scroll-mt-24 items-baseline gap-3.5 border-b border-border px-2 py-3.5 transition-colors target:bg-signal-soft"
            >
              <CiteChip refId={item.ref} label={t("evidence.cite", { ref: item.ref })} />
              <SourceTag kind={item.source_kind} name={item.source_name} />
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                {isHttpUrl(item.url) ? (
                  <a
                    href={item.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-[14.5px] leading-snug break-words text-foreground underline-offset-2 hover:underline"
                  >
                    {item.title}
                  </a>
                ) : (
                  <span className="text-[14.5px] leading-snug break-words">{item.title}</span>
                )}
                <EvidenceMeta item={item} />
              </span>
              {newItemIds.has(item.item_id) && (
                <IdeaBadge tone="marker">{t("evidence.newIn", { version })}</IdeaBadge>
              )}
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
