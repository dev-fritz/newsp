import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import type { OwnProvider } from "@/features/profile/api/profile-api";
import {
  useDeleteOwnProvider,
  useTestOwnProvider,
  useUpdateOwnProvider,
} from "@/features/profile/api/profile-queries";
import { OwnConnectionResult } from "@/features/profile/components/own-connection-result";
import { OwnModelRow } from "@/features/profile/components/own-model-row";
import {
  OwnProviderStatusBadge,
  type OwnProviderStatus,
} from "@/features/profile/components/own-provider-status-badge";
import { ReplaceKeyDialog } from "@/features/profile/components/replace-key-dialog";
import { useOwnErrorToast } from "@/features/profile/hooks/use-own-error-toast";
import { cn } from "@/lib/utils";

type Dialog = "key" | "delete" | null;

function statusOf(p: OwnProvider): OwnProviderStatus {
  if (!p.enabled) return "disabled";
  if (p.last_error) return "error";
  if (!p.last_check_at) return "untested";
  return "connected";
}

type OwnProviderCardProps = {
  provider: OwnProvider;
  /** BYOK is off: the only action left is removing the stored key. */
  readOnly?: boolean;
};

/** One connected provider: state, key hint (never the key), models and actions. */
export function OwnProviderCard({ provider, readOnly = false }: OwnProviderCardProps) {
  const { t, i18n } = useTranslation("profile");
  const toastError = useOwnErrorToast();
  const test = useTestOwnProvider();
  const update = useUpdateOwnProvider();
  const remove = useDeleteOwnProvider();
  const [dialog, setDialog] = useState<Dialog>(null);

  const name = t(`ownProviders.kinds.${provider.kind}`);
  const status = statusOf(provider);
  const checked = provider.last_check_at
    ? new Intl.DateTimeFormat(i18n.language, { dateStyle: "medium", timeStyle: "short" }).format(
        new Date(provider.last_check_at),
      )
    : null;

  function runTest() {
    test.mutate(provider.id, { onError: toastError });
  }

  function confirmDelete() {
    remove.mutate(provider.id, {
      onSuccess: () => toast.success(t("ownProviders.toast.removed", { name })),
      onError: (error) => {
        setDialog(null);
        toastError(error);
      },
    });
  }

  return (
    <article
      className={cn(
        "flex flex-col gap-3 rounded-2xl border bg-card p-4",
        status === "error" ? "border-negative" : "border-border",
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <h3 className="text-[15.5px] font-semibold">{name}</h3>
        <OwnProviderStatusBadge status={status} />
      </div>
      <p className="text-[13px]">
        {provider.has_api_key
          ? t("ownProviders.card.key", { hint: provider.key_hint || "••••" })
          : t("ownProviders.card.noKey")}
        {checked && (
          <span className="text-subtle-foreground">
            {" "}
            · {t("ownProviders.card.checked", { when: checked })}
          </span>
        )}
      </p>
      {status === "error" && provider.last_error && (
        <p className="text-[13px] break-words text-negative">{provider.last_error}</p>
      )}

      {provider.models.length > 0 ? (
        <ul className="flex flex-col divide-y divide-border rounded-lg border border-border">
          {provider.models.map((model) => (
            <OwnModelRow key={model.id} model={model} readOnly={readOnly} />
          ))}
        </ul>
      ) : (
        !readOnly && (
          <p className="text-[13px] text-muted-foreground">{t("ownProviders.card.noModels")}</p>
        )
      )}

      {!readOnly && (test.isPending || test.data) && (
        <OwnConnectionResult provider={provider} result={test.data} pending={test.isPending} />
      )}

      <div className="flex flex-wrap items-center gap-2 pt-1">
        {!readOnly && (
          <>
            <Button
              variant="secondary"
              size="sm"
              onClick={runTest}
              disabled={!provider.enabled || test.isPending}
              className="pointer-coarse:h-11"
            >
              {t("ownProviders.card.test")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setDialog("key")}
              className="pointer-coarse:h-11"
            >
              {t("ownProviders.card.replaceKey")}
            </Button>
          </>
        )}
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setDialog("delete")}
          className="hover:text-negative pointer-coarse:h-11"
        >
          {t("ownProviders.card.remove")}
        </Button>
        {!readOnly && (
          <label className="ml-auto flex items-center gap-2 text-[13px] pointer-coarse:min-h-11">
            <Switch
              checked={provider.enabled}
              disabled={update.isPending}
              onCheckedChange={(enabled) =>
                update.mutate({ id: provider.id, body: { enabled } }, { onError: toastError })
              }
              className="data-checked:bg-signal"
            />
            {t("ownProviders.card.enabled")}
          </label>
        )}
      </div>

      {!readOnly && dialog === "key" && (
        <ReplaceKeyDialog
          provider={provider}
          name={name}
          open
          onOpenChange={(open) => !open && setDialog(null)}
          onReplaced={runTest}
        />
      )}
      <ConfirmDialog
        open={dialog === "delete"}
        onOpenChange={(open) => !open && setDialog(null)}
        title={t("ownProviders.remove.title", { name })}
        description={t("ownProviders.remove.body")}
        confirmLabel={t("ownProviders.remove.confirm")}
        onConfirm={confirmDelete}
        pending={remove.isPending}
      />
    </article>
  );
}
