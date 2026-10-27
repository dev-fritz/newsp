import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";

/** Switch in the signal accent, for enabled/disabled status in admin tables. */
export function StatusSwitch({ className, ...props }: React.ComponentProps<typeof Switch>) {
  return <Switch className={cn("data-checked:bg-signal", className)} {...props} />;
}
