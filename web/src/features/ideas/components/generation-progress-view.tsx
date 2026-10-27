import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";

import { useIdeaGeneration, type GenerationBase } from "../hooks/use-idea-generation";
import { GenerationSteps } from "./generation-steps";

function BaseRows({ base }: { base: GenerationBase }) {
  const { t } = useTranslation("ideas");
  if (base.kind === "report") {
    return (
      <>
        <dt className="text-subtle-foreground">{t("dialog.base")}</dt>
        <dd className="m-0">{base.report.title}</dd>
      </>
    );
  }
  const items =
    base.items > 0 ? t("dialog.itemsPicked", { count: base.items }) : t("dialog.noItems");
  return (
    <>
      <dt className="text-subtle-foreground">{t("dialog.base")}</dt>
      <dd className="m-0">
        {t(`generate.windowOption.d${base.windowDays}`)} · {items}
      </dd>
      {base.focus && (
        <>
          <dt className="text-subtle-foreground">{t("dialog.focusShort")}</dt>
          <dd className="m-0 [overflow-wrap:anywhere]">{base.focus}</dd>
        </>
      )}
    </>
  );
}

/** The dialog while the job runs: it can be closed, the run goes on. */
export function GenerationProgressView() {
  const { t } = useTranslation("ideas");
  const { run, queued, closeDialog } = useIdeaGeneration();
  if (!run) return null;
  return (
    <div className="flex flex-col gap-5">
      <DialogHeader className="pr-8">
        <output
          aria-live="polite"
          className="flex items-center gap-2 text-[13px] font-medium text-signal-ink"
        >
          <span
            aria-hidden
            className="inline-block size-[7px] animate-pulse rounded-full bg-signal motion-reduce:animate-none"
          />
          {queued ? t("dialog.queued") : t("dialog.running")}
        </output>
        <DialogTitle className="font-serif text-[26px] leading-tight font-medium">
          {queued
            ? t("progress.queued", { count: run.count })
            : t("progress.title", { count: run.count })}
        </DialogTitle>
        <DialogDescription>{t("dialog.progressHint")}</DialogDescription>
      </DialogHeader>
      <Skeleton aria-hidden className="h-1 rounded-full" />
      <dl className="m-0 grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 rounded-lg bg-muted px-4 py-3.5 text-[13.5px]">
        <BaseRows base={run.base} />
      </dl>
      <GenerationSteps />
      <DialogFooter className="border-t border-border pt-4">
        <Button
          type="button"
          variant="secondary"
          className="h-9 pointer-coarse:h-11"
          onClick={closeDialog}
        >
          {t("dialog.closeAndFollow")}
        </Button>
      </DialogFooter>
    </div>
  );
}
