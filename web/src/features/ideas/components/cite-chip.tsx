import { cn } from "@/lib/utils";

const chipClass =
  "mx-px inline-flex h-[18px] min-w-5 items-center justify-center rounded-[5px] bg-signal-soft px-[5px] align-[2px] font-mono text-[11px] leading-none font-semibold text-signal-ink";

type CiteChipProps = Omit<React.ComponentProps<"a">, "href"> & {
  refId: string;
  /** Accessible name, e.g. "Evidence 1". */
  label: string;
  /** In-page anchor ("#evidence-E1"); without it the chip is plain text. */
  href?: string;
  className?: string;
};

/** The "E1" citation tag: a link to the evidence entry, or a static label. */
export function CiteChip({ refId, label, href, className, ...props }: CiteChipProps) {
  if (!href) return <span className={cn(chipClass, className)}>{refId}</span>;
  return (
    <a
      {...props}
      href={href}
      aria-label={label}
      className={cn(
        chipClass,
        "no-underline transition-colors hover:bg-signal hover:text-background focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
        className,
      )}
    >
      {refId}
    </a>
  );
}
