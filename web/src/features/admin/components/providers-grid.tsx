import { Plus } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { Provider } from "../types";
import { AddProviderDialog } from "./add-provider-dialog";
import { ProviderCard } from "./provider-card";

/** Provider cards plus the dashed "add provider" card. */
export function ProvidersGrid({ providers }: { providers: Provider[] }) {
  const { t } = useTranslation("admin");
  const [adding, setAdding] = useState(false);
  return (
    <>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(min(260px,100%),1fr))] gap-3.5">
        {providers.map((p) => (
          <ProviderCard key={p.id} provider={p} />
        ))}
        <button
          type="button"
          onClick={() => setAdding(true)}
          className="flex min-h-40 flex-col items-center justify-center gap-2.5 rounded-2xl border border-dashed border-border-strong p-4 text-muted-foreground transition-colors outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <Plus className="size-5" aria-hidden />
          <span className="font-medium text-foreground">{t("ai.add.card")}</span>
          <span className="text-center text-[12.5px]">{t("ai.add.cardHint")}</span>
        </button>
      </div>
      <AddProviderDialog open={adding} onOpenChange={setAdding} />
    </>
  );
}
