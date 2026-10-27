import { useQueryClient } from "@tanstack/react-query";
import { Loader2, RefreshCw } from "lucide-react";
import { useCallback, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";

import { useJobCompletion } from "../api/jobs-queries";
import { sourceKeys, useCollectSource } from "../api/sources-queries";
import { useErrorToast } from "../hooks/use-error-toast";
import { TOUCH_TARGET } from "../lib/touch";
import type { Job, Source } from "../types";

/** Queues a collection, follows the job and refreshes the list when it ends. */
export function CollectSourceButton({ source }: { source: Source }) {
  const { t } = useTranslation("admin");
  const toastError = useErrorToast();
  const queryClient = useQueryClient();
  const collect = useCollectSource();
  const [jobId, setJobId] = useState<string | null>(null);

  const finished = useCallback(
    (job: Job) => {
      setJobId(null);
      void queryClient.invalidateQueries({ queryKey: sourceKeys.all });
      if (job.status === "done") toast.success(t("sources.collect.done", { name: source.name }));
      else toast.error(t("sources.collect.failed", { name: source.name }));
    },
    [queryClient, source.name, t],
  );
  useJobCompletion(jobId, finished);

  const running = collect.isPending || jobId !== null;

  function start() {
    collect.mutate(source.id, {
      onSuccess: (accepted) => {
        toast.info(t("sources.collect.started", { name: source.name }));
        setJobId(accepted.job_id);
      },
      onError: toastError,
    });
  }

  return (
    <Button
      variant="ghost"
      size="icon"
      className={TOUCH_TARGET}
      onClick={start}
      disabled={running}
      aria-busy={running}
      aria-label={t("sources.actions.collect", { name: source.name })}
      title={t("sources.actions.collectTitle")}
    >
      {running ? <Loader2 className="animate-spin" aria-hidden /> : <RefreshCw aria-hidden />}
    </Button>
  );
}
