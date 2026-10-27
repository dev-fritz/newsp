import { useTranslation } from "react-i18next";

import type { PlanStep } from "../schemas/idea-response.schema";
import { IdeaSection } from "./idea-section";
import { RoadmapSteps } from "./roadmap-steps";

/** The MVP plan as a roadmap: one node per period. */
export function IdeaMvpSection({ plan }: { plan: PlanStep[] }) {
  const { t } = useTranslation("ideas");
  if (plan.length === 0) return null;
  return (
    <IdeaSection title={t("sections.mvp")}>
      <RoadmapSteps
        steps={plan.map((step, index) => ({
          key: `${index}-${step.period}`,
          period: step.period,
          title: step.title,
          detail: step.description,
        }))}
      />
    </IdeaSection>
  );
}
