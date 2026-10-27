import { MoreHorizontal } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

import { useDeleteProvider, useTestProvider, useUpdateProvider } from "../api/ai-queries";
import { useErrorToast } from "../hooks/use-error-toast";
import { useFormat } from "../hooks/use-format";
import { providerStatus } from "../lib/provider-status";
import { TOUCH_TARGET } from "../lib/touch";
import type { Provider } from "../types";
import { EditProviderDialog } from "./edit-provider-dialog";
import { ProviderStatusBadge } from "./provider-status-badge";
import { TestProviderDialog } from "./test-provider-dialog";

type Dialog = "test" | "edit" | "key" | "delete" | null;

/** One provider: state, base URL, key hint (never the key) and its actions. */
export function ProviderCard({ provider }: { provider: Provider }) {
  const { t } = useTranslation("admin");
  const format = useFormat();
  const toastError = useErrorToast();
  const test = useTestProvider();
  const update = useUpdateProvider();
  const remove = useDeleteProvider();
  const [dialog, setDialog] = useState<Dialog>(null);

  const status = providerStatus(provider);
  const authError = status === "error" && provider.kind !== "openai_compatible";

  function runTest() {
    test.mutate(provider.id, { onError: toastError });
  }

  function openTest() {
    setDialog("test");
    runTest();
  }

  function toggleEnabled() {
    update.mutate(
      { id: provider.id, body: { enabled: !provider.enabled } },
      { onError: toastError },
    );
  }

  function confirmDelete() {
    remove.mutate(provider.id, {
      onSuccess: () => {
        toast.success(t("ai.toast.deleted", { name: provider.name }));
        setDialog(null);
      },
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
        <div className="flex min-w-0 flex-col gap-0.5">
          <h3 className="truncate text-[15.5px] font-semibold">{provider.name}</h3>
          <span className="text-[12.5px] text-subtle-foreground">
            {t(`ai.kinds.${provider.kind}.label`)}
          </span>
        </div>
        <ProviderStatusBadge status={status} />
      </div>
      {provider.base_url && (
        <span className="font-mono text-xs break-all text-subtle-foreground">
          {provider.base_url}
        </span>
      )}
      {status === "error" && provider.last_error && (
        <p className="text-[13px] text-negative">{provider.last_error}</p>
      )}
      <p className="text-[13px]">
        {t("ai.card.models", { count: provider.models.length })} ·{" "}
        {provider.has_api_key
          ? t("ai.card.key", { hint: provider.key_hint || "••••" })
          : t("ai.card.noKey")}
        {provider.last_check_at && status !== "disabled" && (
          <span className="text-subtle-foreground">
            {" "}
            · {t("ai.card.checked", { when: format.relative(provider.last_check_at) })}
          </span>
        )}
      </p>
      <div className="mt-auto flex items-center gap-1.5 pt-1">
        <Button
          variant="secondary"
          size="sm"
          onClick={openTest}
          disabled={!provider.enabled}
          className="pointer-coarse:h-11"
        >
          {t("ai.card.test")}
        </Button>
        {authError ? (
          <Button size="sm" onClick={() => setDialog("key")} className="pointer-coarse:h-11">
            {t("ai.card.updateKey")}
          </Button>
        ) : (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setDialog("edit")}
            className="pointer-coarse:h-11"
          >
            {t("ai.card.edit")}
          </Button>
        )}
        <DropdownMenu modal={false}>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon-sm"
              className={cn("ml-auto", TOUCH_TARGET)}
              aria-label={t("ai.card.more", { name: provider.name })}
            >
              <MoreHorizontal aria-hidden />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {authError && (
              <DropdownMenuItem onSelect={() => setDialog("edit")}>
                {t("ai.card.edit")}
              </DropdownMenuItem>
            )}
            {!authError && (
              <DropdownMenuItem onSelect={() => setDialog("key")}>
                {t("ai.card.updateKey")}
              </DropdownMenuItem>
            )}
            <DropdownMenuItem onSelect={toggleEnabled}>
              {provider.enabled ? t("ai.card.disable") : t("ai.card.enable")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onSelect={() => setDialog("delete")}>
              {t("ai.card.delete")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <TestProviderDialog
        provider={provider}
        result={test.data}
        pending={test.isPending}
        onRetest={runTest}
        open={dialog === "test"}
        onOpenChange={(open) => !open && setDialog(null)}
      />
      {(dialog === "edit" || dialog === "key") && (
        <EditProviderDialog
          provider={provider}
          keyOnly={dialog === "key"}
          open
          onOpenChange={(open) => !open && setDialog(null)}
        />
      )}
      <ConfirmDialog
        open={dialog === "delete"}
        onOpenChange={(open) => !open && setDialog(null)}
        title={t("ai.delete.title", { name: provider.name })}
        description={t("ai.delete.body")}
        confirmLabel={t("ai.delete.confirm")}
        onConfirm={confirmDelete}
        pending={remove.isPending}
      />
    </article>
  );
}
