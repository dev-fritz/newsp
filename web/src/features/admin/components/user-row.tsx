import { MoreHorizontal } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { TableCell, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";

import { useCreateResetLink, useUpdateUserAccess } from "../api/users-queries";
import { useErrorToast } from "../hooks/use-error-toast";
import { useFormat } from "../hooks/use-format";
import { TOUCH_TARGET } from "../lib/touch";
import type { AdminUser, OneTimeLink } from "../types";
import { LinkDialog } from "./link-dialog";
import { UserAvatar } from "./user-avatar";

type Change = "disable" | "makeAdmin" | "makeMember";

export function UserRow({ user, isSelf }: { user: AdminUser; isSelf: boolean }) {
  const { t } = useTranslation("admin");
  const format = useFormat();
  const toastError = useErrorToast();
  const update = useUpdateUserAccess();
  const reset = useCreateResetLink();
  const [link, setLink] = useState<OneTimeLink | null>(null);
  const [confirming, setConfirming] = useState<Change | null>(null);

  const disabled = user.status === "disabled";

  function apply(body: { role?: AdminUser["role"]; status?: AdminUser["status"] }, done: string) {
    update.mutate(
      { id: user.id, body },
      {
        onSuccess: () => {
          toast.success(t(done, { name: user.name }));
          setConfirming(null);
        },
        onError: (error) => {
          setConfirming(null);
          toastError(error);
        },
      },
    );
  }

  function confirm() {
    if (confirming === "disable") apply({ status: "disabled" }, "users.toast.disabled");
    if (confirming === "makeAdmin") apply({ role: "admin" }, "users.toast.madeAdmin");
    if (confirming === "makeMember") apply({ role: "member" }, "users.toast.madeMember");
  }

  function createLink() {
    reset.mutate(user.id, { onSuccess: setLink, onError: toastError });
  }

  return (
    <TableRow>
      <TableCell>
        <div className="flex items-center gap-3">
          <UserAvatar name={user.name} inverted={user.role === "admin"} muted={disabled} />
          <div className="flex min-w-0 flex-col">
            <span
              className={cn("font-medium", disabled ? "text-muted-foreground" : "text-foreground")}
            >
              {user.name}
              {isSelf && (
                <span className="font-normal text-subtle-foreground"> ({t("users.you")})</span>
              )}
            </span>
            <span className="truncate text-[12.5px] text-subtle-foreground">{user.email}</span>
          </div>
        </div>
      </TableCell>
      <TableCell>
        <Badge
          className={
            user.role === "admin"
              ? "bg-signal-soft text-signal-ink"
              : "bg-secondary text-secondary-foreground"
          }
        >
          {t(`common:roles.${user.role}`)}
        </Badge>
      </TableCell>
      <TableCell>
        <Badge
          className={
            disabled
              ? "border-border-strong bg-transparent text-muted-foreground"
              : "bg-positive-soft text-positive"
          }
        >
          {t(`users.status.${user.status}`)}
        </Badge>
        {user.email_verified === false && (
          <Badge className="mt-1 border-border-strong bg-transparent text-muted-foreground">
            {t("users.emailUnverified")}
          </Badge>
        )}
      </TableCell>
      <TableCell className="text-subtle-foreground">
        {user.last_seen_at ? format.relative(user.last_seen_at) : t("users.neverSeen")}
      </TableCell>
      <TableCell>
        {isSelf ? (
          <div className="text-right text-[12.5px] text-subtle-foreground">—</div>
        ) : (
          <div className="flex items-center justify-end gap-1">
            <Button
              variant="ghost"
              size="sm"
              onClick={createLink}
              disabled={reset.isPending || disabled}
              className="pointer-coarse:h-11"
            >
              {t("users.actions.resetLink")}
            </Button>
            {disabled ? (
              <Button
                variant="ghost"
                size="sm"
                disabled={update.isPending}
                onClick={() => apply({ status: "active" }, "users.toast.reactivated")}
                className="pointer-coarse:h-11"
              >
                {t("users.actions.reactivate")}
              </Button>
            ) : (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setConfirming("disable")}
                className="text-negative hover:text-negative pointer-coarse:h-11"
              >
                {t("users.actions.disable")}
              </Button>
            )}
            <DropdownMenu modal={false}>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  className={TOUCH_TARGET}
                  aria-label={t("users.actions.more", { name: user.name })}
                >
                  <MoreHorizontal aria-hidden />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {user.role === "admin" ? (
                  <DropdownMenuItem onSelect={() => setConfirming("makeMember")}>
                    {t("users.actions.makeMember")}
                  </DropdownMenuItem>
                ) : (
                  <DropdownMenuItem onSelect={() => setConfirming("makeAdmin")}>
                    {t("users.actions.makeAdmin")}
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        )}
        <LinkDialog
          title={t("users.reset.title", { name: user.name })}
          description={t("users.reset.body")}
          link={link}
          onClose={() => setLink(null)}
        />
        <ConfirmDialog
          open={confirming !== null}
          onOpenChange={(open) => !open && setConfirming(null)}
          title={t(`users.confirm.${confirming ?? "disable"}.title`, { name: user.name })}
          description={t(`users.confirm.${confirming ?? "disable"}.body`)}
          confirmLabel={t(`users.confirm.${confirming ?? "disable"}.confirm`)}
          onConfirm={confirm}
          pending={update.isPending}
          destructive={confirming === "disable"}
        />
      </TableCell>
    </TableRow>
  );
}
