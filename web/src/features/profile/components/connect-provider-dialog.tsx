import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

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
import type { OwnProviderKind } from "@/features/profile/api/profile-api";
import {
  useCreateOwnProvider,
  useOwnProviders,
  useTestOwnProvider,
} from "@/features/profile/api/profile-queries";
import { OwnConnectionResult } from "@/features/profile/components/own-connection-result";
import { OwnKeyForm } from "@/features/profile/components/own-key-form";

type ConnectProviderDialogProps = {
  kind: OwnProviderKind;
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

/** Two steps: register the key, then see the test and pick models. Mounted only while open. */
export function ConnectProviderDialog({ kind, open, onOpenChange }: ConnectProviderDialogProps) {
  const { t } = useTranslation(["profile", "common"]);
  const providers = useOwnProviders();
  const create = useCreateOwnProvider();
  const test = useTestOwnProvider();
  const [createdId, setCreatedId] = useState<string | null>(null);

  const name = t(`profile:ownProviders.kinds.${kind}`);
  const provider = createdId
    ? providers.data?.providers.find((p) => p.id === createdId)
    : undefined;

  async function runTest(id: string) {
    try {
      await test.mutateAsync(id);
    } catch {
      // The panel falls back to "not tested yet" with a retry button.
    }
  }

  async function connect(apiKey: string) {
    const created = await create.mutateAsync({ kind, api_key: apiKey });
    toast.success(t("profile:ownProviders.toast.connected", { name }));
    setCreatedId(created.id);
    await runTest(created.id);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[92svh] gap-5 overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle className="font-serif text-2xl font-medium">
            {createdId
              ? t("profile:ownProviders.connect.doneTitle", { name })
              : t("profile:ownProviders.connect.title", { name })}
          </DialogTitle>
          <DialogDescription>
            {createdId
              ? t("profile:ownProviders.connect.doneBody")
              : t("profile:ownProviders.connect.body", { name })}
          </DialogDescription>
        </DialogHeader>
        {!createdId ? (
          <OwnKeyForm
            hint={t("profile:ownProviders.form.apiKeyHint", { name })}
            submitLabel={t("profile:ownProviders.form.connect")}
            pendingLabel={t("profile:ownProviders.form.connecting")}
            onSubmit={connect}
            onCancel={() => onOpenChange(false)}
          />
        ) : !provider ? (
          <Skeleton className="h-32 w-full rounded-xl" aria-hidden />
        ) : (
          <>
            <OwnConnectionResult
              provider={provider}
              result={test.data}
              pending={test.isPending}
              onRetest={() => void runTest(provider.id)}
            />
            <DialogFooter>
              <Button onClick={() => onOpenChange(false)} className="pointer-coarse:h-11">
                {t("profile:ownProviders.connect.done")}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
