import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";

import { useProviders, useTestProvider } from "../api/ai-queries";
import type { ConnectionTest } from "../types";
import { ProviderConnection } from "./provider-connection";
import { ProviderForm } from "./provider-form";

type AddProviderDialogProps = { open: boolean; onOpenChange: (open: boolean) => void };

/** Two steps: register the provider, then see the test and pick models. */
export function AddProviderDialog({ open, onOpenChange }: AddProviderDialogProps) {
  const { t } = useTranslation("admin");
  const providers = useProviders();
  const test = useTestProvider();
  const [created, setCreated] = useState<{ id: string; test: ConnectionTest | null } | null>(null);

  const provider = created ? providers.data?.find((p) => p.id === created.id) : undefined;

  function close(next: boolean) {
    onOpenChange(next);
    if (!next) {
      setCreated(null);
      test.reset();
    }
  }

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="max-h-[92svh] gap-5 overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle className="font-serif text-2xl font-medium">
            {created ? t("ai.add.connectedTitle") : t("ai.add.title")}
          </DialogTitle>
          <DialogDescription>
            {created ? t("ai.add.connectedBody") : t("ai.add.body")}
          </DialogDescription>
        </DialogHeader>
        {created && !provider ? (
          <Skeleton className="h-32 w-full rounded-xl" aria-hidden />
        ) : created && provider ? (
          <>
            <ProviderConnection
              provider={provider}
              result={test.data ?? created.test}
              pending={test.isPending}
              onRetest={() => test.mutate(provider.id)}
            />
            <DialogFooter>
              <Button onClick={() => close(false)} className="pointer-coarse:h-11">
                {t("ai.add.done")}
              </Button>
            </DialogFooter>
          </>
        ) : (
          <ProviderForm
            onConnected={(p, result) => setCreated({ id: p.id, test: result })}
            onCancel={() => close(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
