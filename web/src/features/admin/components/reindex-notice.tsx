import { Loader2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import { useJob } from "../api/jobs-queries";

/** Follows the reindex job started by changing the embedding model. */
export function ReindexNotice({ jobId }: { jobId: string }) {
  const { t } = useTranslation("admin");
  const job = useJob(jobId);
  const status = job.data?.status;
  const running = !status || status === "pending" || status === "running";
  const key = running ? "running" : status === "done" ? "done" : "failed";
  return (
    <output
      aria-live="polite"
      className={cn(
        "flex items-start gap-2.5 rounded-xl p-3 text-[13px] leading-snug",
        key === "failed" ? "bg-negative-soft text-negative" : "bg-warning-soft text-warning",
      )}
    >
      {running && <Loader2 className="mt-px size-4 shrink-0 animate-spin" aria-hidden />}
      <span>{t(`ai.settings.reindex.${key}`)}</span>
    </output>
  );
}
