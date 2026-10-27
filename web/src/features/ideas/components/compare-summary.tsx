import { GitMerge, Pencil, Plus, Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useIdeaFormat } from "../hooks/use-idea-format";
import type { VersionDiff } from "../lib/idea-diff";
import type { IdeaVersion, VersionOrigin } from "../schemas/idea-response.schema";
import { IdeaBadge } from "./idea-badge";

const ICONS = {
  merged: GitMerge,
  deepened: Sparkles,
  generated: Plus,
  manual: Pencil,
} as const satisfies Record<VersionOrigin, typeof Plus>;

function Stat({ value, label }: { value: string; label: string }) {
  return (
    <span className="text-[13.5px]">
      <strong className="font-semibold">{value}</strong>{" "}
      <span className="text-muted-foreground">{label}</span>
    </span>
  );
}

type Props = { target: IdeaVersion; diff: VersionDiff };

/** What produced the target version and how much changed. */
export function CompareSummary({ target, diff }: Props) {
  const { t } = useTranslation("ideas");
  const format = useIdeaFormat();
  const Icon = ICONS[target.origin];
  return (
    <section
      aria-labelledby="summary-title"
      className="flex flex-col gap-4 rounded-xl border border-border bg-card p-[22px]"
    >
      <h2 id="summary-title" className="sr-only">
        {t("compare.summary")}
      </h2>
      <div className="flex flex-wrap items-center gap-2.5">
        <IdeaBadge tone="signal">
          <Icon aria-hidden />
          {t(`compare.origin.${target.origin}`)}
        </IdeaBadge>
        <span className="font-mono text-xs text-subtle-foreground">
          {[format.dateTime(target.created_at), target.model_name].filter(Boolean).join(" · ")}
        </span>
      </div>
      {target.change_note && (
        <p className="m-0 font-serif text-xl leading-normal break-words">{target.change_note}</p>
      )}
      <div className="flex flex-wrap gap-x-5 gap-y-2 border-t border-border pt-3">
        <Stat
          value={String(diff.changedCount)}
          label={t("compare.changedSections", { count: diff.changedCount })}
        />
        <Stat
          value={`+${diff.evidenceAdded}`}
          label={t("compare.evidenceAdded", { count: diff.evidenceAdded })}
        />
        {diff.evidenceRemoved > 0 && (
          <Stat
            value={`−${diff.evidenceRemoved}`}
            label={t("compare.evidenceRemoved", { count: diff.evidenceRemoved })}
          />
        )}
        <Stat
          value={String(diff.unchangedCount)}
          label={t("compare.unchangedSections", { count: diff.unchangedCount })}
        />
      </div>
    </section>
  );
}
