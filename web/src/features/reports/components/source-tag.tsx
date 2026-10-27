import { sourceTag } from "../format";

/** Tiny monochrome masthead tag of a source kind ("HN", "TN", "r/"…). */
export function SourceTag({ kind }: { kind: string | undefined }) {
  return (
    <span
      aria-hidden
      className="inline-flex h-5 min-w-[26px] shrink-0 items-center justify-center rounded-[4px] border border-border-strong bg-card px-[5px] font-mono text-[10.5px] leading-none font-semibold tracking-[0.02em] text-foreground"
    >
      {sourceTag(kind)}
    </span>
  );
}
