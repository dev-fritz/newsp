import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";

import type { Source } from "../types";

/** OK / Error / Paused / waiting for the first run, with the last error under it. */
export function SourceStatusBadge({ source }: { source: Source }) {
  const { t } = useTranslation("admin");
  let label: string;
  let tone: string;
  if (!source.enabled) {
    label = t("sources.status.paused");
    tone = "border-border-strong bg-transparent text-muted-foreground";
  } else if (source.last_error) {
    label = t("sources.status.error");
    tone = "bg-negative-soft text-negative";
  } else if (!source.last_run_at) {
    label = t("sources.status.waiting");
    tone = "border-border-strong bg-transparent text-muted-foreground";
  } else {
    label = t("sources.status.ok");
    tone = "bg-positive-soft text-positive";
  }
  return (
    <span className="flex flex-col items-start gap-1">
      <Badge className={tone}>
        <span aria-hidden className="size-1.5 rounded-full bg-current" />
        {label}
      </Badge>
      {source.enabled && source.last_error && (
        <span className="max-w-64 text-xs text-negative">{source.last_error}</span>
      )}
    </span>
  );
}
