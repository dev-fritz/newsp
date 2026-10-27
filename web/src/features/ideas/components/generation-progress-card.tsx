import { useTranslation } from "react-i18next";

import { GenerationSteps } from "./generation-steps";

/** Placeholder card shown in the grid while a generation job runs; opens the dialog. */
export function GenerationProgressCard({
  count,
  queued,
  onOpen,
}: {
  count: number;
  queued: boolean;
  onOpen: () => void;
}) {
  const { t } = useTranslation("ideas");
  return (
    <button
      type="button"
      onClick={onOpen}
      className="flex min-h-[280px] flex-col gap-4 rounded-xl border border-dashed border-border-strong bg-card p-[22px] text-left outline-none hover:border-border-strong hover:shadow-md focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <output aria-live="polite" className="flex items-center gap-2 text-[13px] font-medium">
        <span
          aria-hidden
          className="inline-block size-[7px] animate-pulse rounded-full bg-signal motion-reduce:animate-none"
        />
        {queued ? t("progress.queued", { count }) : t("progress.title", { count })}
      </output>
      <GenerationSteps as="span" />
      <span className="block text-xs text-subtle-foreground">{t("progress.hint")}</span>
      <span className="mt-auto flex flex-col gap-2" aria-hidden>
        <span className="block h-3.5 w-4/5 animate-pulse rounded-md bg-muted motion-reduce:animate-none" />
        <span className="block h-3.5 w-[55%] animate-pulse rounded-md bg-muted motion-reduce:animate-none" />
      </span>
      <span className="block text-xs text-signal-ink">{t("dialog.openProgress")}</span>
    </button>
  );
}
