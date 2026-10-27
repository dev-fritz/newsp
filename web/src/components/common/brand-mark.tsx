import { cn } from "@/lib/utils";

/** The newsp wordmark: serif name with the signal-colored period. */
export function BrandMark({ className }: { className?: string }) {
  return (
    <span className={cn("font-serif font-semibold tracking-tight", className)}>
      newsp<span className="text-signal">.</span>
    </span>
  );
}
