import { useId } from "react";
import { useTranslation } from "react-i18next";

import { splitStep } from "../lib/idea-text";
import { RoadmapSteps } from "./roadmap-steps";

/** Architecture steps as a roadmap, in data-flow order. */
export function ArchitectureFlow({ steps }: { steps: string[] }) {
  const { t } = useTranslation("ideas");
  const headingId = useId();
  if (steps.length === 0) return null;
  return (
    <div className="flex flex-col gap-3.5 rounded-xl bg-muted p-5 sm:p-6">
      <span id={headingId} className="kicker">
        {t("how.architecture")}
      </span>
      <RoadmapSteps
        aria-labelledby={headingId}
        steps={steps.map((step, index) => ({ key: `${index}-${step}`, ...splitStep(step) }))}
      />
    </div>
  );
}
