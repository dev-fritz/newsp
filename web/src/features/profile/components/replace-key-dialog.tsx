import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { OwnProvider } from "@/features/profile/api/profile-api";
import { useUpdateOwnProvider } from "@/features/profile/api/profile-queries";
import { OwnKeyForm } from "@/features/profile/components/own-key-form";

type ReplaceKeyDialogProps = {
  provider: OwnProvider;
  /** Name shown to the user (translated kind). */
  name: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called after the key was replaced, so the caller can re-test the connection. */
  onReplaced: () => void;
};

export function ReplaceKeyDialog({
  provider,
  name,
  open,
  onOpenChange,
  onReplaced,
}: ReplaceKeyDialogProps) {
  const { t } = useTranslation("profile");
  const update = useUpdateOwnProvider();

  async function replace(apiKey: string) {
    await update.mutateAsync({ id: provider.id, body: { api_key: apiKey } });
    toast.success(t("ownProviders.toast.keyReplaced", { name }));
    onOpenChange(false);
    onReplaced();
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[92svh] gap-5 overflow-y-auto sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="font-serif text-2xl font-medium">
            {t("ownProviders.replace.title", { name })}
          </DialogTitle>
          <DialogDescription>{t("ownProviders.replace.body")}</DialogDescription>
        </DialogHeader>
        <OwnKeyForm
          hint={t("ownProviders.form.apiKeyHint", { name })}
          submitLabel={t("ownProviders.replace.submit")}
          pendingLabel={t("ownProviders.replace.submitting")}
          onSubmit={replace}
          onCancel={() => onOpenChange(false)}
        />
      </DialogContent>
    </Dialog>
  );
}
