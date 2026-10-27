import { cn } from "@/lib/utils";

import { KIND_TAG } from "../lib/source-kinds";
import type { SourceKind } from "../types";

/** Tiny monochrome masthead tag of a connector (HN, TN, r/, YT, RSS). */
export function SourceKindTag({ kind, className }: { kind: SourceKind; className?: string }) {
  return (
    <span
      aria-hidden
      className={cn(
        "inline-flex h-5 min-w-7 shrink-0 items-center justify-center rounded bg-muted px-1.5 font-mono text-[10.5px] font-semibold tracking-wide text-muted-foreground",
        className,
      )}
    >
      {KIND_TAG[kind]}
    </span>
  );
}
