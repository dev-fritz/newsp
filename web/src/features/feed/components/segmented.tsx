import { cn } from "@/lib/utils";

type SegmentedProps<V extends string> = {
  label: string;
  value: V;
  options: readonly { value: V; label: string }[];
  onChange: (value: V) => void;
  className?: string;
};

/** Pressed-button group for picking one value, like the design's segmented control. */
export function Segmented<V extends string>({
  label,
  value,
  options,
  onChange,
  className,
}: SegmentedProps<V>) {
  return (
    <fieldset
      className={cn(
        "m-0 inline-flex max-w-full min-w-0 gap-0.5 rounded-lg border border-border bg-muted p-[3px]",
        className,
      )}
    >
      <legend className="sr-only">{label}</legend>
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          aria-pressed={option.value === value}
          onClick={() => onChange(option.value)}
          className={cn(
            "inline-flex min-h-[30px] items-center justify-center rounded-[7px] px-3 text-[13px] font-medium whitespace-nowrap text-muted-foreground transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50 pointer-coarse:min-h-11",
            "aria-pressed:bg-card aria-pressed:text-foreground aria-pressed:shadow-sm",
          )}
        >
          {option.label}
        </button>
      ))}
    </fieldset>
  );
}
