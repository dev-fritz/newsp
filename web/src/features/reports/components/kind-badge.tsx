import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

import type { ReportKind } from "../types";

/** "Diário" (muted) or "Sob demanda" (signal). */
export function KindBadge({
  kind,
  className,
}: {
  kind: ReportKind | undefined;
  className?: string;
}) {
  const { t } = useTranslation("reports");
  const manual = kind === "manual";
  return (
    <Badge
      variant="secondary"
      className={cn(
        "h-5 rounded-md text-[11px]",
        manual ? "bg-signal-soft text-signal-ink" : "bg-muted text-muted-foreground",
        className,
      )}
    >
      {t(manual ? "kind.manual" : "kind.scheduled")}
    </Badge>
  );
}
