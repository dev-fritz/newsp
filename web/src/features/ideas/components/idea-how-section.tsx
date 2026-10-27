import { useTranslation } from "react-i18next";

import type { IdeaContent } from "../schemas/idea-response.schema";
import { ArchitectureFlow } from "./architecture-flow";
import { IdeaSection } from "./idea-section";
import { MarkdownText } from "./markdown-text";

/** How to build it: the approach, the architecture flow and the suggested stack. */
export function IdeaHowSection({ content }: { content: IdeaContent }) {
  const { t } = useTranslation("ideas");
  return (
    <IdeaSection title={t("sections.how")}>
      <MarkdownText value={content.how} />
      <ArchitectureFlow steps={content.architecture_steps} />
      {content.stack.length > 0 && (
        <div className="flex flex-col gap-2.5">
          <span className="kicker">{t("how.stack")}</span>
          <ul className="m-0 flex list-none flex-wrap gap-2 p-0">
            {content.stack.map((item, index) => (
              <li
                key={`${index}-${item}`}
                className="inline-flex min-h-7 items-center rounded-full border border-border bg-card px-2.5 text-[13px] break-words"
              >
                {item}
              </li>
            ))}
          </ul>
        </div>
      )}
    </IdeaSection>
  );
}
