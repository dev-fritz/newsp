import { useTranslation } from "react-i18next";

import { useIdeaFormat } from "../hooks/use-idea-format";
import type { Idea } from "../schemas/idea-response.schema";
import { CitationProvider } from "./citation-context";
import { IdeaAudienceSection } from "./idea-audience-section";
import { IdeaBadge } from "./idea-badge";
import { IdeaCompetitorsSection } from "./idea-competitors-section";
import { IdeaEvidenceSection } from "./idea-evidence-section";
import { IdeaHowSection } from "./idea-how-section";
import { IdeaMvpSection } from "./idea-mvp-section";
import { IdeaSection } from "./idea-section";
import { IdeaSummaryStrip } from "./idea-summary-strip";
import { MarkdownText } from "./markdown-text";

type Props = {
  idea: Idea;
  /** Evidence item ids that the previous version did not have. */
  newEvidenceIds: ReadonlySet<string>;
};

/** The reading column of an idea: title, strip and every content section. */
export function IdeaArticle({ idea, newEvidenceIds }: Props) {
  const { t } = useTranslation("ideas");
  const format = useIdeaFormat();
  const { current } = idea;
  const { content } = current;
  const refs = new Map(current.evidence.map((item) => [item.ref, item] as const));

  return (
    <article className="flex min-w-0 flex-col gap-[30px]">
      <header className="flex flex-col gap-4">
        <h1 className="max-w-[28ch] font-serif text-[34px] leading-[1.06] font-medium tracking-[-0.02em] text-balance sm:text-5xl">
          {current.title}
        </h1>
        <p className="max-w-[60ch] font-serif text-lg leading-[1.45] text-pretty text-muted-foreground italic sm:text-[22px]">
          {current.summary}
        </p>
        <div className="flex flex-wrap gap-1.5 pt-1 sm:gap-2">
          <IdeaBadge tone="outline">
            {t("detail.difficulty", { level: t(`difficulty.${current.difficulty}`) })}
          </IdeaBadge>
          {current.effort_weeks !== null && (
            <IdeaBadge tone="outline">{t("effort", { count: current.effort_weeks })}</IdeaBadge>
          )}
          <IdeaBadge tone="outline">
            {t("evidenceCount", { count: current.evidence.length })}
          </IdeaBadge>
          <IdeaBadge tone="outline">
            {t("detail.createdAt", { date: format.date(idea.created_at) })}
          </IdeaBadge>
        </div>
      </header>

      <IdeaSummaryStrip content={content} />

      <CitationProvider value={refs}>
        <IdeaSection title={t("sections.whyNow")}>
          <MarkdownText value={content.why_now} />
        </IdeaSection>
        <IdeaAudienceSection audience={content.audience} />
        <IdeaHowSection content={content} />
        {content.differentiator.trim() !== "" && (
          <IdeaSection title={t("sections.differentiator")}>
            <MarkdownText value={content.differentiator} />
          </IdeaSection>
        )}
        <IdeaMvpSection plan={content.mvp_plan} />
        {content.monetization.trim() !== "" && (
          <IdeaSection title={t("sections.monetization")}>
            <MarkdownText value={content.monetization} />
          </IdeaSection>
        )}
        <IdeaCompetitorsSection competitors={content.competitors} />
      </CitationProvider>

      <IdeaEvidenceSection
        evidence={current.evidence}
        newItemIds={newEvidenceIds}
        version={current.version}
      />
    </article>
  );
}
