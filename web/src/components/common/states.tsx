import { AlertCircle, Lock } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import type { ApiError } from "@/lib/api-error";

/** Explains an empty list and offers the next step. */
export function EmptyState({
  title,
  body,
  action,
}: {
  title: string;
  body?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-start gap-3 rounded-2xl border border-dashed border-border-strong p-8">
      <h2 className="font-serif text-2xl font-medium">{title}</h2>
      {body && <p className="max-w-xl text-muted-foreground">{body}</p>}
      {action}
    </div>
  );
}

/** Error with a retry; 403 shows the no-permission state instead. */
export function ErrorState({ error, onRetry }: { error?: ApiError | null; onRetry?: () => void }) {
  const { t } = useTranslation();
  if (error?.status === 403) {
    return (
      <div
        role="alert"
        className="flex flex-col items-start gap-2 rounded-2xl border border-border p-8"
      >
        <Lock className="size-5 text-muted-foreground" aria-hidden />
        <h2 className="font-serif text-2xl font-medium">{t("states.forbiddenTitle")}</h2>
        <p className="text-muted-foreground">{t("states.forbiddenBody")}</p>
      </div>
    );
  }
  return (
    <div
      role="alert"
      className="flex flex-col items-start gap-3 rounded-2xl border border-border p-8"
    >
      <AlertCircle className="size-5 text-negative" aria-hidden />
      <h2 className="font-serif text-2xl font-medium">{t("states.errorTitle")}</h2>
      <p className="text-muted-foreground">
        {error
          ? t(`errors.${error.code}`, { defaultValue: t("states.errorBody") })
          : t("states.errorBody")}
      </p>
      {onRetry && (
        <Button variant="outline" onClick={onRetry}>
          {t("actions.retry")}
        </Button>
      )}
    </div>
  );
}

/** Placeholder rows while a list loads. */
export function ListSkeleton({ rows = 4 }: { rows?: number }) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-4" aria-busy="true" aria-label={t("states.loading")}>
      {Array.from({ length: rows }, (_, i) => (
        <Skeleton key={i} className="h-24 w-full rounded-2xl" />
      ))}
    </div>
  );
}

/** Temporary screen body for routes that land in later tasks. */
export function ComingSoon() {
  const { t } = useTranslation();
  return <EmptyState title={t("states.comingSoon")} />;
}
