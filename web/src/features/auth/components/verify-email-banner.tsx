import { MailWarning } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";

import { useResendVerificationEmail } from "../api/auth-queries";

/** Shown to signed-in users whose e-mail is not confirmed yet. */
export function VerifyEmailBanner({ email }: { email: string }) {
  const { t } = useTranslation("auth");
  const resend = useResendVerificationEmail();

  const onResend = () =>
    resend.mutate(undefined, {
      onSuccess: () => toast.success(t("verifyBanner.sent")),
      onError: (error) => {
        if (error.code === "RATE_LIMITED") toast.warning(t("verifyBanner.RATE_LIMITED"));
        else if (error.code === "EMAIL_ALREADY_VERIFIED")
          toast.info(t("verifyBanner.EMAIL_ALREADY_VERIFIED"));
        else toast.error(t("verifyBanner.failed"));
      },
    });

  return (
    <output className="mb-6 flex w-full flex-col gap-3 rounded-lg border border-border bg-card px-4 py-3 text-sm sm:flex-row sm:items-center sm:justify-between">
      <p className="flex items-start gap-2.5 text-muted-foreground">
        <MailWarning aria-hidden className="mt-0.5 size-4 shrink-0 text-signal" />
        <span className="min-w-0 break-words">{t("verifyBanner.text", { email })}</span>
      </p>
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="shrink-0 self-start sm:self-auto"
        onClick={onResend}
        disabled={resend.isPending}
      >
        {resend.isPending ? t("verifyBanner.resending") : t("verifyBanner.resend")}
      </Button>
    </output>
  );
}
