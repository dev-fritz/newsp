import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const ideaBadgeVariants = cva(
  "inline-flex h-[22px] w-fit shrink-0 items-center gap-1 rounded-md border border-transparent px-2 text-xs font-medium whitespace-nowrap [&>svg]:size-3",
  {
    variants: {
      tone: {
        neutral: "bg-muted text-muted-foreground",
        outline: "border-border-strong bg-transparent text-muted-foreground",
        signal: "bg-signal-soft text-signal-ink",
        marker: "bg-marker text-marker-ink",
        positive: "bg-positive-soft text-positive",
        negative: "bg-negative-soft text-negative",
        warning: "bg-warning-soft text-warning",
      },
    },
    defaultVariants: { tone: "neutral" },
  },
);

type IdeaBadgeProps = React.ComponentProps<"span"> & VariantProps<typeof ideaBadgeVariants>;

/** Small status or metadata label. */
export function IdeaBadge({ tone, className, ...props }: IdeaBadgeProps) {
  return <span className={cn(ideaBadgeVariants({ tone }), className)} {...props} />;
}
