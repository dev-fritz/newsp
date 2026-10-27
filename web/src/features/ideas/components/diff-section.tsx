import { useId } from "react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import type { SectionDiff } from "../lib/idea-diff";
import { DiffRows } from "./diff-rows";
import { DiffText } from "./diff-text";
import { EvidenceDiff } from "./evidence-diff";
import { IdeaBadge } from "./idea-badge";

function SectionStatus({ section }: { section: SectionDiff }) {
  const { t } = useTranslation("ideas");
  if (!section.changed) return <IdeaBadge>{t("compare.unchanged")}</IdeaBadge>;
  if (section.body.type === "evidence") {
    const { added, removed } = section.body;
    return (
      <span className="flex gap-1.5">
        {added.length > 0 && <IdeaBadge tone="positive">+{added.length}</IdeaBadge>}
        {removed.length > 0 && <IdeaBadge tone="negative">−{removed.length}</IdeaBadge>}
      </span>
    );
  }
  return <IdeaBadge tone="warning">{t("compare.changed")}</IdeaBadge>;
}

function MetaBody({
  body,
  onlyChanges,
}: {
  body: Extract<SectionDiff["body"], { type: "meta" }>;
  onlyChanges: boolean;
}) {
  const { t } = useTranslation("ideas");
  const difficulty = (value: string) => t(`difficulty.${value}`);
  const effort = (value: number | null) => (value === null ? "—" : t("effort", { count: value }));
  const lines = [
    {
      key: "difficulty",
      label: t("edit.difficulty"),
      before: difficulty(body.difficulty.before),
      after: difficulty(body.difficulty.after),
    },
    {
      key: "effort",
      label: t("edit.effort"),
      before: effort(body.effort.before),
      after: effort(body.effort.after),
    },
  ].filter((line) => !onlyChanges || line.before !== line.after);
  return (
    <dl className="m-0 flex flex-col gap-1.5 text-[15px]">
      {lines.map((line) => (
        <div key={line.key} className="flex flex-wrap gap-x-3">
          <dt className="text-muted-foreground">{line.label}</dt>
          <dd className="m-0">
            {line.before === line.after ? (
              line.after
            ) : (
              <DiffText
                ops={[
                  { type: "remove", text: line.before },
                  { type: "equal", text: " " },
                  { type: "add", text: line.after },
                ]}
              />
            )}
          </dd>
        </div>
      ))}
    </dl>
  );
}

type Props = { section: SectionDiff; onlyChanges: boolean };

/** One section of the comparison: heading, change status and its marked-up content. */
export function DiffSection({ section, onlyChanges }: Props) {
  const { t } = useTranslation("ideas");
  const headingId = useId();
  const { body } = section;
  return (
    <section
      aria-labelledby={headingId}
      className="flex flex-col gap-3 border-b border-border pb-[22px]"
    >
      <div className="flex items-center justify-between gap-2.5">
        <h2 id={headingId} className="font-serif text-2xl font-semibold">
          {t(`compare.sections.${section.id}`)}
        </h2>
        <SectionStatus section={section} />
      </div>
      <div className={cn(!section.changed && "text-muted-foreground")}>
        {body.type === "text" && (
          <p className="m-0 font-serif text-[19px] leading-[1.6] break-words whitespace-pre-line">
            <DiffText ops={body.ops} />
          </p>
        )}
        {body.type === "rows" && (
          <DiffRows sectionId={section.id} rows={body.rows} onlyChanges={onlyChanges} />
        )}
        {body.type === "meta" && <MetaBody body={body} onlyChanges={onlyChanges} />}
        {body.type === "evidence" && (
          <EvidenceDiff
            added={body.added}
            removed={body.removed}
            kept={body.kept}
            onlyChanges={onlyChanges}
          />
        )}
      </div>
    </section>
  );
}
