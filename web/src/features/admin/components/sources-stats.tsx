import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import { useFormat } from "../hooks/use-format";
import type { Source } from "../types";

function Stat({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div className="flex flex-col gap-1 bg-card px-4 py-4">
      <span className="kicker">{label}</span>
      <span className={cn("font-serif text-3xl font-medium", tone)}>{value}</span>
    </div>
  );
}

/** Active, failing, items in 24 h and next collection, computed from the list. */
export function SourcesStats({ sources }: { sources: Source[] }) {
  const { t } = useTranslation("admin");
  const format = useFormat();
  const active = sources.filter((s) => s.enabled);
  const failing = active.filter((s) => s.last_error).length;
  const items = sources.reduce((sum, s) => sum + s.items_24h, 0);
  const times = active.flatMap((s) => (s.next_run_at ? [new Date(s.next_run_at).getTime()] : []));
  const upcoming = times.length > 0 ? Math.min(...times) : undefined;
  return (
    <div className="grid grid-cols-[repeat(auto-fit,minmax(min(170px,100%),1fr))] gap-px overflow-hidden rounded-2xl border border-border bg-border">
      <Stat label={t("sources.stats.active")} value={String(active.length)} />
      <Stat
        label={t("sources.stats.failing")}
        value={String(failing)}
        tone={failing > 0 ? "text-negative" : undefined}
      />
      <Stat label={t("sources.stats.items")} value={format.number(items, 0)} />
      <Stat
        label={t("sources.stats.next")}
        value={upcoming === undefined ? "—" : format.relative(new Date(upcoming))}
      />
    </div>
  );
}
