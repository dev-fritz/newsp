import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";

import type { ProviderStatus } from "../lib/provider-status";

const TONES: Record<ProviderStatus, string> = {
  connected: "bg-positive-soft text-positive",
  error: "bg-negative-soft text-negative",
  disabled: "border-border-strong bg-transparent text-muted-foreground",
  untested: "border-border-strong bg-transparent text-muted-foreground",
};

export function ProviderStatusBadge({ status }: { status: ProviderStatus }) {
  const { t } = useTranslation("admin");
  return (
    <Badge className={TONES[status]}>
      <span aria-hidden className="size-1.5 rounded-full bg-current" />
      {t(`ai.status.${status}`)}
    </Badge>
  );
}
