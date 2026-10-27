import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

type Option<T extends string> = { value: T; label: string; icon?: React.ReactNode };

type SegmentedControlProps<T extends string> = {
  labelledBy: string;
  value: T;
  options: Option<T>[];
  onChange: (value: T) => void;
};

/** Single-choice pill group (a radio group in disguise, keyboard accessible). */
export function SegmentedControl<T extends string>({
  labelledBy,
  value,
  options,
  onChange,
}: SegmentedControlProps<T>) {
  return (
    <ToggleGroup
      type="single"
      variant="outline"
      spacing={0}
      value={value}
      aria-labelledby={labelledBy}
      className="self-start"
      onValueChange={(next) => {
        const match = options.find((o) => o.value === next);
        if (match) onChange(match.value);
      }}
    >
      {options.map((o) => (
        <ToggleGroupItem
          key={o.value}
          value={o.value}
          className="h-10 gap-1.5 px-4 pointer-coarse:h-11"
        >
          {o.icon}
          {o.label}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  );
}
