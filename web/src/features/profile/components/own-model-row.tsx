import { Trash2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import type { OwnModel } from "@/features/profile/api/profile-api";
import { useDeleteOwnModel, useUpdateOwnModel } from "@/features/profile/api/profile-queries";
import { useOwnErrorToast } from "@/features/profile/hooks/use-own-error-toast";
import { cn } from "@/lib/utils";

type OwnModelRowProps = {
  model: OwnModel;
  /** BYOK is off: show the model only, without switch or remove. */
  readOnly?: boolean;
};

/** One registered chat model: tokens used in the last 7 days, enable switch and remove. */
export function OwnModelRow({ model, readOnly = false }: OwnModelRowProps) {
  const { t, i18n } = useTranslation("profile");
  const toastError = useOwnErrorToast();
  const update = useUpdateOwnModel();
  const remove = useDeleteOwnModel();
  const [confirming, setConfirming] = useState(false);

  const name = model.display_name || model.name;
  const tokens = new Intl.NumberFormat(i18n.language, {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(model.tokens_last_7d);

  function confirmDelete() {
    remove.mutate(model.id, {
      onSuccess: () => {
        toast.success(t("ownProviders.toast.modelRemoved", { name }));
        setConfirming(false);
      },
      onError: (error) => {
        setConfirming(false);
        toastError(error);
      },
    });
  }

  return (
    <li className="flex items-center justify-between gap-3 px-3 py-2">
      <div className={cn("flex min-w-0 flex-col", !model.enabled && "text-muted-foreground")}>
        <span className="truncate font-mono text-[13px] font-medium" title={name}>
          {name}
        </span>
        <span className="text-xs text-subtle-foreground">
          {t("ownProviders.models.tokens", { tokens })}
        </span>
      </div>
      {!readOnly && (
        <div className="flex shrink-0 items-center gap-2">
          <Switch
            checked={model.enabled}
            disabled={update.isPending}
            onCheckedChange={(enabled) =>
              update.mutate({ id: model.id, body: { enabled } }, { onError: toastError })
            }
            aria-label={t("ownProviders.models.enabled", { name })}
            className="data-checked:bg-signal"
          />
          <Button
            variant="ghost"
            size="icon"
            className="hover:text-negative pointer-coarse:min-h-11 pointer-coarse:min-w-11"
            onClick={() => setConfirming(true)}
            aria-label={t("ownProviders.models.remove", { name })}
          >
            <Trash2 aria-hidden />
          </Button>
        </div>
      )}
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={t("ownProviders.models.removeTitle", { name })}
        description={t("ownProviders.models.removeBody")}
        confirmLabel={t("ownProviders.models.removeConfirm")}
        onConfirm={confirmDelete}
        pending={remove.isPending}
      />
    </li>
  );
}
