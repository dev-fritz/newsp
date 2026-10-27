import { useEffect, useEffectEvent, useRef } from "react";

import { readGenerationResult, type IdeaJob } from "../api/ideas-api";
import { isJobUnreadable, useIdeaJobQuery } from "../api/ideas-queries";

export type JobOutcome =
  { kind: "success"; created: string[]; merged: string[] } | { kind: "failure"; code: string };

const DEFAULT_FAILURE = "AI_FAILURE";

function outcomeOf(job: IdeaJob): JobOutcome {
  if (job.status !== "done") return { kind: "failure", code: job.error ?? DEFAULT_FAILURE };
  const result = readGenerationResult(job.result);
  if (result.status !== "done") return { kind: "failure", code: result.error ?? DEFAULT_FAILURE };
  return { kind: "success", created: result.created, merged: result.merged };
}

/**
 * Follows a generation or deepen job and reports its outcome once, when it
 * reaches a final status (or cannot be read at all).
 */
export function useIdeaJob(jobId: string | null, onFinished: (outcome: JobOutcome) => void) {
  const query = useIdeaJobQuery(jobId);
  const handled = useRef<string | null>(null);
  const finish = useEffectEvent(onFinished);
  const job = query.data;
  // A transient read error keeps following the job; the worker may still finish it.
  const failedToRead = isJobUnreadable(query.error, query.errorUpdateCount);

  useEffect(() => {
    if (jobId === null || handled.current === jobId) return;
    if (failedToRead) {
      handled.current = jobId;
      finish({ kind: "failure", code: query.error?.code ?? DEFAULT_FAILURE });
      return;
    }
    if (!job || job.status === "pending" || job.status === "running") return;
    handled.current = jobId;
    finish(outcomeOf(job));
  }, [jobId, job, failedToRead, query.error]);

  return {
    job,
    running:
      jobId !== null &&
      !failedToRead &&
      (!job || job.status === "pending" || job.status === "running"),
  };
}
