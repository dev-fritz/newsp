import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { ConfirmDialog } from "@/components/common/confirm-dialog";

import { useDeleteReport } from "../api/reports-queries";

type DeleteReportDialogProps = {
  reportId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Runs after the report is gone, e.g. to leave the reader. */
  onDeleted?: () => void;
};

/** Confirms the permanent deletion of a report and its citations. */
export function DeleteReportDialog({
  reportId,
  open,
  onOpenChange,
  onDeleted,
}: DeleteReportDialogProps) {
  const { t } = useTranslation("reports");
  const remove = useDeleteReport();

  function confirm() {
    remove.mutate(reportId, {
      onSuccess: () => {
        toast.success(t("delete.success"));
        onOpenChange(false);
        onDeleted?.();
      },
      onError: (error) => {
        onOpenChange(false);
        toast.error(
          t([
            `delete.errors.${error.code}`,
            `common:errors.${error.code}`,
            "delete.errors.generic",
          ]),
        );
      },
    });
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("delete.title")}
      description={t("delete.description")}
      confirmLabel={t("delete.confirm")}
      onConfirm={confirm}
      pending={remove.isPending}
    />
  );
}
