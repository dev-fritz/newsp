import { Link } from "react-router";
import { useTranslation } from "react-i18next";

import { useIdeaFormat } from "../hooks/use-idea-format";
import type { IdeaListItem } from "../schemas/idea-response.schema";
import { IdeaBadge } from "./idea-badge";
import { IdeaStatusBadge } from "./idea-status-badge";

/** An idea in a list or grid; the whole card opens the idea. */
export function IdeaCard({ idea }: { idea: IdeaListItem }) {
  const { t } = useTranslation("ideas");
  const format = useIdeaFormat();
  return (
    <Link
      to={`/ideas/${idea.id}`}
      className="group flex min-h-[280px] flex-col gap-3 rounded-xl border border-border bg-card p-[22px] text-foreground no-underline transition duration-150 outline-none hover:-translate-y-px hover:border-border-strong hover:shadow-sm focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <span className="flex items-center justify-between gap-2">
        <IdeaStatusBadge
          status={idea.status}
          current_version={idea.current_version}
          last_origin={idea.last_origin}
        />
        <span className="font-mono text-xs text-subtle-foreground">
          {format.relative(idea.updated_at)}
        </span>
      </span>
      <h2 className="font-serif text-[23px] leading-[1.2] font-medium text-balance">
        {idea.title}
      </h2>
      <p className="text-sm leading-[1.55] text-muted-foreground">{idea.summary}</p>
      {idea.primary_audience && (
        <span className="text-[13px]">
          <span className="text-subtle-foreground">{t("card.audience")} </span>
          {idea.primary_audience}
        </span>
      )}
      <span className="mt-auto flex flex-wrap gap-1.5 border-t border-border pt-3.5">
        <IdeaBadge tone="outline">{t(`difficulty.${idea.difficulty}`)}</IdeaBadge>
        {idea.effort_weeks !== null && (
          <IdeaBadge tone="outline">{t("effort", { count: idea.effort_weeks })}</IdeaBadge>
        )}
        <IdeaBadge tone="outline">{t("evidenceCount", { count: idea.evidence_count })}</IdeaBadge>
      </span>
    </Link>
  );
}
