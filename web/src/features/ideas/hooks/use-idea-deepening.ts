import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { useDeepenIdea, useInvalidateIdeas } from "../api/ideas-queries";
import type { DeepenIdeaValues } from "../schemas/idea.schema";
import { useErrorMessage } from "./use-error-message";
import { useIdeaJob } from "./use-idea-job";

/** Queues a deeper version of an idea and follows the job; the idea refreshes when it ends. */
export function useIdeaDeepening(ideaId: string) {
  const { t } = useTranslation("ideas");
  const errorMessage = useErrorMessage();
  const invalidate = useInvalidateIdeas();
  const deepen = useDeepenIdea(ideaId);
  const [jobId, setJobId] = useState<string | null>(null);

  const { running } = useIdeaJob(jobId, (outcome) => {
    setJobId(null);
    void invalidate();
    if (outcome.kind === "success") toast.success(t("deepen.done"));
    // Nothing broke: the model just had nothing to add, and no version was stored.
    else if (outcome.code === "NO_CHANGES") toast.warning(errorMessage(outcome.code));
    else toast.error(errorMessage(outcome.code));
  });

  function submit(values: DeepenIdeaValues, onStarted: () => void) {
    deepen.mutate(
      { aspects: values.aspects, instruction: values.instruction || undefined },
      {
        onSuccess: (id) => {
          setJobId(id);
          onStarted();
        },
        onError: (error) => toast.error(errorMessage(error.code)),
      },
    );
  }

  return { submit, isStarting: deepen.isPending, running };
}

export type IdeaDeepening = ReturnType<typeof useIdeaDeepening>;
