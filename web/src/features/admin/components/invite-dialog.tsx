import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";

import { FormField } from "@/components/common/form-field";
import { isApiError } from "@/lib/api-error";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

import { useCreateInvite } from "../api/users-queries";
import { useErrorToast } from "../hooks/use-error-toast";
import { inviteFormSchema, type InviteFormValues } from "../schemas/invite.schema";
import type { InviteLink, Role } from "../types";
import { OneTimeLink } from "./one-time-link";

type InviteDialogProps = { open: boolean; onOpenChange: (open: boolean) => void };

/** Picks the role, creates the invite and shows its link once. */
export function InviteDialog({ open, onOpenChange }: InviteDialogProps) {
  const { t } = useTranslation("admin");
  const toastError = useErrorToast();
  const create = useCreateInvite();
  const [role, setRole] = useState<Role>("member");
  const [link, setLink] = useState<InviteLink | null>(null);
  const [apiError, setApiError] = useState<string | null>(null);
  const form = useForm<InviteFormValues>({
    resolver: zodResolver(inviteFormSchema),
    defaultValues: { email: "" },
  });

  function close(next: boolean) {
    onOpenChange(next);
    if (!next) {
      setLink(null);
      setRole("member");
      setApiError(null);
      form.reset();
    }
  }

  function submit(v: InviteFormValues) {
    setApiError(null);
    create.mutate(
      { role, ...(v.email ? { email: v.email } : {}) },
      {
        onSuccess: setLink,
        onError: (error) => {
          if (
            isApiError(error) &&
            (error.code === "EMAIL_TAKEN" || error.code === "MAILER_NOT_CONFIGURED")
          )
            setApiError(error.code);
          else toastError(error);
        },
      },
    );
  }

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="gap-5 sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="font-serif text-2xl font-medium">
            {link ? t("users.invite.readyTitle") : t("users.invite.title")}
          </DialogTitle>
          <DialogDescription>
            {link ? t("users.invite.readyBody") : t("users.invite.body")}
          </DialogDescription>
        </DialogHeader>
        {link ? (
          <>
            {link.emailed && link.email && (
              <p className="rounded-xl bg-positive-soft p-3 text-[13px] leading-snug text-positive">
                {t("users.invite.emailed", { email: link.email })}
              </p>
            )}
            <OneTimeLink link={link} />
            <div className="flex justify-end">
              <Button onClick={() => close(false)} className="pointer-coarse:h-11">
                {t("users.invite.done")}
              </Button>
            </div>
          </>
        ) : (
          <form onSubmit={form.handleSubmit(submit)} noValidate className="flex flex-col gap-5">
            <div className="flex flex-col gap-2">
              <span id="invite-role" className="text-sm leading-none font-medium">
                {t("users.invite.role")}
              </span>
              <ToggleGroup
                type="single"
                variant="outline"
                value={role}
                onValueChange={(v) => {
                  if (v === "admin" || v === "member") setRole(v);
                }}
                aria-labelledby="invite-role"
                spacing={0}
              >
                <ToggleGroupItem value="member" className="px-4 pointer-coarse:h-11">
                  {t("common:roles.member")}
                </ToggleGroupItem>
                <ToggleGroupItem value="admin" className="px-4 pointer-coarse:h-11">
                  {t("common:roles.admin")}
                </ToggleGroupItem>
              </ToggleGroup>
              <p className="text-xs text-subtle-foreground">{t(`users.invite.roleHint.${role}`)}</p>
            </div>
            <FormField
              label={t("users.invite.email")}
              type="email"
              autoComplete="off"
              hint={t("users.invite.emailHint")}
              error={form.formState.errors.email?.message}
              {...form.register("email", { onChange: () => setApiError(null) })}
            />
            {apiError && (
              <p className="text-xs text-negative" role="alert">
                {t(`errors.${apiError}`)}
              </p>
            )}
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="ghost"
                onClick={() => close(false)}
                className="pointer-coarse:h-11"
              >
                {t("common:actions.cancel")}
              </Button>
              <Button type="submit" disabled={create.isPending} className="pointer-coarse:h-11">
                {t("users.invite.create")}
              </Button>
            </div>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
