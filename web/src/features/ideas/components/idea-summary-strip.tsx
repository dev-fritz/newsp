import { useTranslation } from "react-i18next";

import { firstSentence } from "../lib/idea-text";
import type { IdeaContent } from "../schemas/idea-response.schema";

function primaryAudience(content: IdeaContent): string {
  const segment = content.audience.find((a) => a.kind === "primary") ?? content.audience[0];
  return segment?.label ?? "";
}

/** The three-column "why / for whom / how" strip under the title. */
export function IdeaSummaryStrip({ content }: { content: IdeaContent }) {
  const { t } = useTranslation("ideas");
  const columns = [
    { key: "why", text: firstSentence(content.why_now) },
    { key: "who", text: primaryAudience(content) },
    { key: "how", text: firstSentence(content.how) },
  ];
  return (
    <dl className="m-0 grid grid-cols-[repeat(auto-fit,minmax(min(200px,100%),1fr))] border-y border-t-2 border-y-border border-t-foreground">
      {columns.map((column) => (
        <div
          key={column.key}
          className="flex flex-col gap-2 border-b border-border py-4 last:border-b-0 sm:border-r sm:border-b-0 sm:px-5 sm:first:pl-0 sm:last:border-r-0 sm:last:pr-0"
        >
          <dt className="kicker">{t(`strip.${column.key}`)}</dt>
          <dd className="m-0 text-[14.5px] leading-normal">{column.text}</dd>
        </div>
      ))}
    </dl>
  );
}
