import { useTranslation } from "react-i18next";

import { Skeleton } from "@/components/ui/skeleton";

/** Placeholder while a report loads. */
export function ReportReaderSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      aria-busy="true"
      aria-label={t("states.loading")}
      className="mx-auto flex max-w-[720px] flex-col gap-6 px-5 pt-8 pb-16 sm:px-10 sm:pt-12"
    >
      <Skeleton className="h-3 w-56" />
      <Skeleton className="h-24 w-full" />
      <Skeleton className="h-16 w-full" />
      <Skeleton className="h-28 w-full rounded-xl" />
      <div className="flex flex-col gap-3">
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-4/5" />
      </div>
    </div>
  );
}
