import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

import { getJob } from "./jobs-api";
import type { Job } from "../types";

export const jobKeys = {
  all: ["admin", "jobs"] as const,
  detail: (id: string) => [...jobKeys.all, "detail", id] as const,
};

export function isFinalJob(status: Job["status"]): boolean {
  return status === "done" || status === "failed" || status === "canceled";
}

/** Polls a queued job until it reaches a final status. */
export function useJob(jobId: string | null) {
  return useQuery({
    queryKey: jobKeys.detail(jobId ?? ""),
    queryFn: () => getJob(jobId ?? ""),
    enabled: jobId !== null,
    staleTime: 0,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status && isFinalJob(status) ? false : 2000;
    },
  });
}

/** Like useJob, and calls onFinished once when the job ends. */
export function useJobCompletion(jobId: string | null, onFinished: (job: Job) => void) {
  const query = useJob(jobId);
  const notified = useRef<string | null>(null);
  const job = query.data;
  useEffect(() => {
    if (!job || !isFinalJob(job.status) || notified.current === job.id) return;
    notified.current = job.id;
    onFinished(job);
  }, [job, onFinished]);
  return query;
}
