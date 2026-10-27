import { useTranslation } from "react-i18next";

import { useIdeaFormat } from "../hooks/use-idea-format";
import type { Idea } from "../schemas/idea-response.schema";

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex justify-between gap-3">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="m-0 min-w-0 text-right break-words">{children}</dd>
    </div>
  );
}

/** Which model wrote the current version, and how it came to be. */
export function IdeaGenerationCard({ idea }: { idea: Idea }) {
  const { t } = useTranslation("ideas");
  const format = useIdeaFormat();
  const { current } = idea;
  return (
    <section
      aria-labelledby="generation-title"
      className="flex flex-col gap-2.5 rounded-xl border border-border bg-card p-5 text-[13px]"
    >
      <h2 id="generation-title" className="kicker text-foreground">
        {t("generation.title")}
      </h2>
      <dl className="m-0 flex flex-col gap-2.5">
        <Row label={t("generation.model")}>
          <span className="font-mono text-xs">{current.model_name ?? t("generation.manual")}</span>
        </Row>
        <Row label={t("generation.origin")}>{t(`origin.${current.origin}`)}</Row>
        <Row label={t("generation.updated")}>{format.dateTime(current.created_at)}</Row>
      </dl>
    </section>
  );
}
