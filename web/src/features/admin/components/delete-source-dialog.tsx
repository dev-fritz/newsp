import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { isApiError } from "@/lib/api-error";

import { useDeleteSource } from "../api/sources-queries";
import { useErrorToast } from "../hooks/use-error-toast";
import type { Source } from "../types";

type DeleteSourceDialogProps = {
  source: Source | null;
  onClose: () => void;
};

/** Confirms deletion; explains the cited-items conflict and points to disabling. */
export function DeleteSourceDialog({ source, onClose }: DeleteSourceDialogProps) {
  const { t } = useTranslation("admin");
  const toastError = useErrorToast();
  const remove = useDeleteSource();

  function confirm() {
    if (!source) return;
    remove.mutate(source.id, {
      onSuccess: () => {
        toast.success(t("sources.toast.deleted", { name: source.name }));
        onClose();
      },
      onError: (error) => {
        onClose();
        if (isApiError(error) && error.code === "SOURCE_HAS_CITED_ITEMS")
          toast.error(t("sources.delete.hasCitedItems", { name: source.name }));
        else toastError(error);
      },
    });
  }

  return (
    <ConfirmDialog
      open={source !== null}
      onOpenChange={(open) => !open && onClose()}
      title={t("sources.delete.title", { name: source?.name ?? "" })}
      description={t("sources.delete.body")}
      confirmLabel={t("sources.delete.confirm")}
      onConfirm={confirm}
      pending={remove.isPending}
    />
  );
}
