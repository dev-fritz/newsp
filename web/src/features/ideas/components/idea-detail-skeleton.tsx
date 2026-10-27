import { useTranslation } from "react-i18next";

import { Skeleton } from "@/components/ui/skeleton";

/** Placeholder with the shape of an idea while it loads. */
export function IdeaDetailSkeleton() {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-6" aria-busy="true" aria-label={t("states.loading")}>
      <Skeleton className="h-6 w-64 max-w-full" />
      <Skeleton className="h-14 w-full max-w-2xl" />
      <Skeleton className="h-16 w-full max-w-xl" />
      <Skeleton className="h-24 w-full" />
      <Skeleton className="h-40 w-full" />
    </div>
  );
}
