import { use } from "react";
import { useTranslation } from "react-i18next";

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { useHoverOpen } from "@/hooks/use-hover-open";
import { cn } from "@/lib/utils";

import { useCompactViewport } from "../hooks/use-compact-viewport";
import { CitationDetails } from "./citation-details";
import { CitationsContext } from "./citations-context";

const CHIP_CLASS = cn(
  "relative mx-0.5 inline-flex h-[18px] min-w-5 items-center justify-center rounded-[5px] bg-signal-soft px-1.5 align-[2px] font-mono text-[11px] leading-none font-semibold text-signal-ink",
  "transition-colors hover:bg-signal hover:text-background focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none aria-expanded:bg-signal aria-expanded:text-background",
  // 44px touch target without inflating the chip.
  "pointer-coarse:after:absolute pointer-coarse:after:-inset-x-1.5 pointer-coarse:after:-inset-y-3 pointer-coarse:after:content-['']",
);

/**
 * A `[n]` citation: a small chip that previews the cited item on hover and opens
 * it on click (popover, or bottom sheet on touch).
 */
export function CitationChip({ reference }: { reference: number }) {
  const { t } = useTranslation("reports");
  const citation = use(CitationsContext).get(reference);
  const compact = useCompactViewport();
  const hover = useHoverOpen();

  if (!citation)
    return <span className="font-mono text-[0.8em] text-subtle-foreground">[{reference}]</span>;

  const chip = (
    <button
      type="button"
      className={CHIP_CLASS}
      aria-label={t("citation.open", { ref: reference })}
    >
      {reference}
    </button>
  );

  if (compact) {
    return (
      <Sheet>
        <SheetTrigger asChild>{chip}</SheetTrigger>
        <SheetContent
          side="bottom"
          showCloseButton={false}
          aria-describedby={undefined}
          className="gap-0 rounded-t-[18px] px-5 pt-2.5 pb-[calc(20px+env(safe-area-inset-bottom))]"
        >
          <span aria-hidden className="mx-auto mb-3.5 h-1 w-10 rounded-sm bg-border-strong" />
          <CitationDetails citation={citation} variant="sheet" />
        </SheetContent>
      </Sheet>
    );
  }

  return (
    <Popover open={hover.open} onOpenChange={hover.setOpen}>
      <PopoverTrigger asChild {...hover.triggerProps}>
        {chip}
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className="w-[340px] max-w-[calc(100vw-2rem)] gap-0 p-4"
        {...hover.contentProps}
      >
        <CitationDetails citation={citation} variant="popover" />
      </PopoverContent>
    </Popover>
  );
}
