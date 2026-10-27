import { GitMerge, Pencil, Plus, Sparkles } from "lucide-react";

import { cn } from "@/lib/utils";

import type { VersionOrigin } from "../schemas/idea-response.schema";

const ICONS = {
  merged: GitMerge,
  deepened: Sparkles,
  generated: Plus,
  manual: Pencil,
} as const satisfies Record<VersionOrigin, typeof Plus>;

type Props = { origin: VersionOrigin; current?: boolean };

/** Round marker of a version in the timeline; the current one is filled. */
export function VersionOriginIcon({ origin, current }: Props) {
  const Icon = ICONS[origin];
  return (
    <span
      aria-hidden
      className={cn(
        "inline-flex size-7 items-center justify-center rounded-full border",
        current
          ? "border-signal bg-signal text-background"
          : "border-border-strong bg-muted text-foreground",
      )}
    >
      <Icon className="size-3.5" />
    </span>
  );
}
