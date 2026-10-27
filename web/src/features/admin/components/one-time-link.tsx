import { Check, Copy } from "lucide-react";
import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

import { useFormat } from "../hooks/use-format";
import type { OneTimeLink as OneTimeLinkData } from "../types";

/** Shows a one-time link (invite or password reset) with a copy button. */
export function OneTimeLink({ link }: { link: OneTimeLinkData }) {
  const { t } = useTranslation("admin");
  const format = useFormat();
  const inputId = useId();
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(link.url);
      setCopied(true);
      toast.success(t("common:actions.copied"));
    } catch {
      toast.error(t("link.copyFailed"));
    }
  }

  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={inputId}>{t("link.label")}</Label>
      <div className="flex gap-2">
        <Input
          id={inputId}
          readOnly
          value={link.url}
          onFocus={(e) => e.currentTarget.select()}
          className="h-10 min-w-0 font-mono text-[12.5px]"
        />
        <Button onClick={copy} className="h-10 shrink-0 pointer-coarse:h-11" aria-live="polite">
          {copied ? <Check aria-hidden /> : <Copy aria-hidden />}
          {copied ? t("common:actions.copied") : t("common:actions.copy")}
        </Button>
      </div>
      <p className="text-xs text-subtle-foreground">
        {t("link.hint", { expires: format.dateTime(link.expires_at) })}
      </p>
    </div>
  );
}
