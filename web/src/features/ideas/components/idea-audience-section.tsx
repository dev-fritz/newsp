import { useTranslation } from "react-i18next";

import type { AudienceKind, AudienceSegment } from "../schemas/idea-response.schema";
import { IdeaBadge } from "./idea-badge";
import { IdeaSection } from "./idea-section";

const TONE = {
  primary: "positive",
  secondary: "neutral",
  out_of_scope: "negative",
} as const satisfies Record<AudienceKind, "positive" | "neutral" | "negative">;

/** Who the idea is for, who it also serves and who it leaves out. */
export function IdeaAudienceSection({ audience }: { audience: AudienceSegment[] }) {
  const { t } = useTranslation("ideas");
  if (audience.length === 0) return null;
  return (
    <IdeaSection title={t("sections.audience")}>
      <ul className="m-0 grid list-none grid-cols-[repeat(auto-fit,minmax(min(220px,100%),1fr))] gap-3 p-0">
        {audience.map((segment, index) => (
          <li
            key={`${segment.kind}-${segment.label}-${index}`}
            className="flex flex-col items-start gap-1.5 rounded-lg border border-border bg-card p-4"
          >
            <IdeaBadge tone={TONE[segment.kind]}>{t(`audience.${segment.kind}`)}</IdeaBadge>
            <span className="text-[15px] font-semibold">{segment.label}</span>
            {segment.description && (
              <span className="text-[13.5px] text-muted-foreground">{segment.description}</span>
            )}
          </li>
        ))}
      </ul>
    </IdeaSection>
  );
}
