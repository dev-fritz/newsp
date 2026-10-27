import { RadioGroup as RadioGroupPrimitive } from "radix-ui";
import { useId } from "react";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

export type OptionCard = {
  value: string;
  title: string;
  description?: string;
  /** Short monochrome tag shown before the title. */
  tag?: string;
  badge?: string;
};

type OptionCardGroupProps = {
  label: string;
  value: string;
  onValueChange: (value: string) => void;
  options: OptionCard[];
  disabled?: boolean;
  /** Minimum card width in px; cards wrap into as many columns as fit. */
  minWidth?: number;
};

/** A radio group drawn as selectable cards (arrow keys move the selection). */
export function OptionCardGroup({
  label,
  value,
  onValueChange,
  options,
  disabled,
  minWidth = 140,
}: OptionCardGroupProps) {
  const labelId = useId();
  return (
    <div className="flex flex-col gap-2.5">
      <span id={labelId} className="text-sm leading-none font-medium">
        {label}
      </span>
      <RadioGroupPrimitive.Root
        aria-labelledby={labelId}
        value={value}
        onValueChange={onValueChange}
        disabled={disabled}
        className="grid gap-2"
        style={{ gridTemplateColumns: `repeat(auto-fit, minmax(min(${minWidth}px, 100%), 1fr))` }}
      >
        {options.map((o) => (
          <RadioGroupPrimitive.Item
            key={o.value}
            value={o.value}
            className={cn(
              "flex flex-col items-start gap-1.5 rounded-xl border border-border bg-card p-3 text-left transition-colors outline-none hover:bg-muted pointer-coarse:min-h-11",
              "focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50",
              "data-[state=checked]:border-signal data-[state=checked]:bg-signal-soft data-[state=checked]:ring-1 data-[state=checked]:ring-signal",
            )}
          >
            <span className="flex w-full items-center justify-between gap-2">
              <span className="flex min-w-0 items-center gap-2">
                {o.tag && (
                  <span className="inline-flex h-5 min-w-6 items-center justify-center rounded bg-muted px-1 font-mono text-[10.5px] font-semibold text-muted-foreground">
                    {o.tag}
                  </span>
                )}
                <span className="text-sm font-medium">{o.title}</span>
              </span>
              {o.badge && (
                <Badge className="h-5 bg-positive-soft text-[11px] text-positive">{o.badge}</Badge>
              )}
            </span>
            {o.description && (
              <span className="text-xs text-subtle-foreground">{o.description}</span>
            )}
          </RadioGroupPrimitive.Item>
        ))}
      </RadioGroupPrimitive.Root>
    </div>
  );
}
