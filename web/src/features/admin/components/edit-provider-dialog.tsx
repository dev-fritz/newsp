import { useTranslation } from "react-i18next";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

import type { Provider } from "../types";
import { ProviderEditForm } from "./provider-edit-form";

type EditProviderDialogProps = {
  provider: Provider;
  /** Only the API key (the "update key" action). */
  keyOnly?: boolean;
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export function EditProviderDialog({
  provider,
  keyOnly,
  open,
  onOpenChange,
}: EditProviderDialogProps) {
  const { t } = useTranslation("admin");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[92svh] gap-5 overflow-y-auto sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="font-serif text-2xl font-medium">
            {keyOnly
              ? t("ai.edit.keyTitle", { name: provider.name })
              : t("ai.edit.title", { name: provider.name })}
          </DialogTitle>
          <DialogDescription>
            {keyOnly ? t("ai.edit.keyBody") : t("ai.edit.body")}
          </DialogDescription>
        </DialogHeader>
        <ProviderEditForm
          provider={provider}
          keyOnly={keyOnly}
          onDone={() => onOpenChange(false)}
          onCancel={() => onOpenChange(false)}
        />
      </DialogContent>
    </Dialog>
  );
}
