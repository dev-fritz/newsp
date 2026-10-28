import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import type { OwnProviderKind } from "@/features/profile/api/profile-api";

type ConnectProviderCardProps = { kind: OwnProviderKind; onConnect: () => void };

/** Placeholder for a kind the user has not connected yet. */
export function ConnectProviderCard({ kind, onConnect }: ConnectProviderCardProps) {
  const { t } = useTranslation("profile");
  const name = t(`ownProviders.kinds.${kind}`);
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-dashed border-border-strong p-4">
      <div className="flex min-w-0 flex-col gap-0.5">
        <h3 className="text-[15.5px] font-semibold">{name}</h3>
        <span className="text-[12.5px] text-subtle-foreground">
          {t("ownProviders.card.notConnected")}
        </span>
      </div>
      <Button variant="outline" size="sm" onClick={onConnect} className="pointer-coarse:h-11">
        {t("ownProviders.card.connect", { name })}
      </Button>
    </div>
  );
}
