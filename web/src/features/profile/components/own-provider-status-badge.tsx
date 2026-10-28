import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";

export type OwnProviderStatus = "connected" | "error" | "untested" | "disabled";

const TONES: Record<OwnProviderStatus, string> = {
  connected: "bg-positive-soft text-positive",
  error: "bg-negative-soft text-negative",
  disabled: "border-border-strong bg-transparent text-muted-foreground",
  untested: "border-border-strong bg-transparent text-muted-foreground",
};

export function OwnProviderStatusBadge({ status }: { status: OwnProviderStatus }) {
  const { t } = useTranslation("profile");
  return (
    <Badge className={TONES[status]}>
      <span aria-hidden className="size-1.5 rounded-full bg-current" />
      {t(`ownProviders.status.${status}`)}
    </Badge>
  );
}
