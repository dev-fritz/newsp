import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { ErrorState } from "@/components/common/states";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

import { useInvites, useRevokeInvite } from "../api/users-queries";
import { useErrorToast } from "../hooks/use-error-toast";
import { useFormat } from "../hooks/use-format";
import { errorOf } from "../lib/error-of";
import type { Invite } from "../types";

/** Pending invites, each revocable. The link itself is never shown again. */
export function InvitesList() {
  const { t } = useTranslation("admin");
  const format = useFormat();
  const toastError = useErrorToast();
  const invites = useInvites();
  const revoke = useRevokeInvite();
  const [revoking, setRevoking] = useState<Invite | null>(null);

  function confirm() {
    if (!revoking) return;
    revoke.mutate(revoking.id, {
      onSuccess: () => {
        toast.success(t("users.toast.revoked"));
        setRevoking(null);
      },
      onError: (error) => {
        setRevoking(null);
        toastError(error);
      },
    });
  }

  return (
    <section aria-labelledby="pending-title" className="flex flex-col gap-3">
      <h2 id="pending-title" className="font-serif text-2xl font-medium">
        {t("users.invites.title")}
      </h2>
      {invites.isPending && <Skeleton className="h-16 w-full rounded-xl" aria-hidden />}
      {invites.isError && (
        <ErrorState error={errorOf(invites.error)} onRetry={() => void invites.refetch()} />
      )}
      {invites.data && invites.data.length === 0 && (
        <p className="text-sm text-muted-foreground">{t("users.invites.empty")}</p>
      )}
      {invites.data?.map((invite) => (
        <div
          key={invite.id}
          className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-border bg-card px-4 py-3.5"
        >
          <div className="flex flex-col gap-0.5">
            <span className="font-medium">{t(`users.invites.${invite.role}`)}</span>
            {invite.email && (
              <span className="text-[12.5px] break-all text-muted-foreground">{invite.email}</span>
            )}
            <span className="text-[12.5px] text-subtle-foreground">
              {t("users.invites.meta", {
                created: format.relative(invite.created_at),
                expires: format.relative(invite.expires_at),
              })}
            </span>
          </div>
          <Button
            variant="ghost"
            size="sm"
            className="text-negative hover:text-negative pointer-coarse:h-11"
            onClick={() => setRevoking(invite)}
          >
            {t("users.invites.revoke")}
          </Button>
        </div>
      ))}
      <ConfirmDialog
        open={revoking !== null}
        onOpenChange={(open) => !open && setRevoking(null)}
        title={t("users.invites.revokeTitle")}
        description={t("users.invites.revokeBody")}
        confirmLabel={t("users.invites.revoke")}
        onConfirm={confirm}
        pending={revoke.isPending}
      />
    </section>
  );
}
