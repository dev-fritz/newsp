import { CheckCircle2, CircleDashed, Loader2, XCircle } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import type { ConnectionTest, OwnProvider } from "@/features/profile/api/profile-api";
import { OwnFoundModels } from "@/features/profile/components/own-found-models";
import { cn } from "@/lib/utils";

type OwnConnectionResultProps = {
  /** The live provider (its models list updates as models are added). */
  provider: OwnProvider;
  result: ConnectionTest | undefined;
  pending: boolean;
  /** Shows the retest button; omit when the caller has its own. */
  onRetest?: () => void;
};

/** Result of a connection test plus the models found, ready to be added. */
export function OwnConnectionResult({
  provider,
  result,
  pending,
  onRetest,
}: OwnConnectionResultProps) {
  const { t } = useTranslation("profile");
  const names = result?.models ?? [];
  const state = pending ? "running" : !result ? "idle" : result.ok ? "ok" : "failed";

  return (
    <div className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <output
          aria-live="polite"
          className={cn(
            "flex items-center gap-2 text-sm font-semibold",
            state === "ok" && "text-positive",
            state === "failed" && "text-negative",
            (state === "running" || state === "idle") && "font-medium text-muted-foreground",
          )}
        >
          {state === "running" && <Loader2 className="size-4 animate-spin" aria-hidden />}
          {state === "idle" && <CircleDashed className="size-4" aria-hidden />}
          {state === "ok" && <CheckCircle2 className="size-4" aria-hidden />}
          {state === "failed" && <XCircle className="size-4" aria-hidden />}
          {state === "ok"
            ? t("ownProviders.test.ok", { count: names.length })
            : t(`ownProviders.test.${state}`)}
        </output>
        {onRetest && (
          <Button
            size="sm"
            variant="secondary"
            onClick={onRetest}
            disabled={pending}
            className="pointer-coarse:h-11"
          >
            {state === "idle" ? t("ownProviders.test.run") : t("ownProviders.test.again")}
          </Button>
        )}
      </div>
      {state === "failed" && (
        <p className="text-sm text-negative">{result?.error || t("ownProviders.test.noDetail")}</p>
      )}
      {state === "ok" && names.length === 0 && (
        <p className="text-sm text-muted-foreground">{t("ownProviders.test.noModels")}</p>
      )}
      {state === "ok" && names.length > 0 && <OwnFoundModels provider={provider} names={names} />}
    </div>
  );
}
