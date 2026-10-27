import { useTranslation } from "react-i18next";

import { engagementParts, formatCompact } from "../format";
import type { Citation } from "../types";

/** Returns a formatter for "412 pts · 87 comentários" style engagement text. */
export function useEngagementLabel(): (citation: Citation) => string {
  const { t, i18n } = useTranslation("reports");
  return (citation) => {
    const { score, comments } = engagementParts(citation);
    const parts: string[] = [];
    if (score !== undefined) {
      parts.push(
        t([`engagement.score.${citation.source_kind ?? ""}`, "engagement.score.default"], {
          value: formatCompact(score, i18n.language),
        }),
      );
    }
    if (comments !== undefined) {
      parts.push(t("engagement.comments", { value: formatCompact(comments, i18n.language) }));
    }
    return parts.join(" · ");
  };
}
