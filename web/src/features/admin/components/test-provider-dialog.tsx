import { useTranslation } from "react-i18next";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

import type { ConnectionTest, Provider } from "../types";
import { ProviderConnection } from "./provider-connection";

type TestProviderDialogProps = {
  provider: Provider;
  result: ConnectionTest | undefined;
  pending: boolean;
  onRetest: () => void;
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

/** "Test connection" of a provider card: found models and one-click adding. */
export function TestProviderDialog({
  provider,
  result,
  pending,
  onRetest,
  open,
  onOpenChange,
}: TestProviderDialogProps) {
  const { t } = useTranslation("admin");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[92svh] gap-5 overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle className="font-serif text-2xl font-medium">
            {t("ai.test.title", { name: provider.name })}
          </DialogTitle>
          <DialogDescription>{t("ai.test.body")}</DialogDescription>
        </DialogHeader>
        <ProviderConnection
          provider={provider}
          result={result}
          pending={pending}
          onRetest={onRetest}
        />
      </DialogContent>
    </Dialog>
  );
}
