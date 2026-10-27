import { useTranslation } from "react-i18next";

import { useIdeaFormat } from "../hooks/use-idea-format";
import type { Idea } from "../schemas/idea-response.schema";

/** Explains that similar candidates were absorbed here instead of becoming duplicates. */
export function IdeaDedupCard({ idea }: { idea: Idea }) {
  const { t } = useTranslation("ideas");
  const format = useIdeaFormat();
  if (idea.merged_candidates <= 0) return null;
  const merges = idea.versions
    .filter((version) => version.origin === "merged")
    .toSorted((a, b) => b.version - a.version);
  return (
    <section
      aria-labelledby="dedup-title"
      className="flex flex-col gap-3 rounded-xl border border-border bg-card p-5"
    >
      <h2 id="dedup-title" className="kicker text-foreground">
        {t("dedup.title")}
      </h2>
      <p className="text-[13.5px] leading-normal">
        {t("dedup.body", { count: idea.merged_candidates })}
      </p>
      {merges.length > 0 && (
        <ul className="m-0 flex list-none flex-col gap-2 p-0">
          {merges.map((merge) => (
            <li key={merge.version} className="flex justify-between gap-2 text-[13px]">
              <span className="text-muted-foreground">
                {t("dedup.merged", {
                  version: merge.version,
                  date: format.date(merge.created_at),
                })}
              </span>
              {merge.similarity !== null && (
                <span className="font-mono text-xs">
                  <span className="sr-only">{t("dedup.similarity")} </span>
                  {format.decimal(merge.similarity)}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
      <span className="text-[12.5px] text-subtle-foreground">{t("dedup.hint")}</span>
    </section>
  );
}
