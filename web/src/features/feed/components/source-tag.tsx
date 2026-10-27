import { sourceTag } from "../lib/item-text";

/** Tiny monochrome tag for the kind of source an item came from. */
export function SourceTag({ kind, name }: { kind: string | undefined; name?: string }) {
  return (
    <span
      title={name || undefined}
      className="inline-flex h-5 min-w-[26px] shrink-0 items-center justify-center rounded-sm border border-border-strong bg-card px-[5px] font-mono text-[10.5px] leading-none font-semibold tracking-[0.02em] text-foreground"
    >
      {sourceTag(kind)}
    </span>
  );
}
