import {
  useInfiniteQuery,
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
  type InfiniteData,
} from "@tanstack/react-query";

import type { ApiError } from "@/lib/api-error";

import type { Idea, IdeaListItem, IdeaStatus, IdeaVersion } from "../schemas/idea-response.schema";
import type { IdeaStatusFilter } from "../schemas/idea.schema";
import {
  deepenIdea,
  editIdea,
  fetchIdea,
  fetchIdeaVersion,
  fetchIdeas,
  fetchJob,
  generateIdeas,
  updateIdeaStatus,
  type DeepenIdeaRequest,
  type EditIdeaRequest,
  type GenerateIdeasRequest,
  type IdeaJob,
  type IdeaPage,
} from "./ideas-api";

export const IDEAS_PAGE_SIZE = 12;

export const ideaKeys = {
  all: ["ideas"] as const,
  lists: () => [...ideaKeys.all, "list"] as const,
  list: (status: IdeaStatusFilter, q: string) => [...ideaKeys.lists(), { status, q }] as const,
  recent: (limit: number) => [...ideaKeys.lists(), "recent", limit] as const,
  details: () => [...ideaKeys.all, "detail"] as const,
  detail: (id: string) => [...ideaKeys.details(), id] as const,
  version: (id: string, version: number) => [...ideaKeys.detail(id), "version", version] as const,
  job: (jobId: string) => ["idea-jobs", jobId] as const,
};

/** Ideas of a status filter, loaded page by page (the server sorts by last update). */
export function useIdeas(status: IdeaStatusFilter, q: string) {
  return useInfiniteQuery<
    IdeaPage,
    ApiError,
    InfiniteData<IdeaPage, number>,
    readonly unknown[],
    number
  >({
    queryKey: ideaKeys.list(status, q),
    initialPageParam: 0,
    queryFn: ({ pageParam }) =>
      fetchIdeas({ status, q, limit: IDEAS_PAGE_SIZE, offset: pageParam }),
    getNextPageParam: (last) => {
      const next = last.meta.offset + last.data.length;
      return last.data.length > 0 && next < last.meta.total ? next : undefined;
    },
  });
}

/** The latest active ideas, for the home screen. */
export function useRecentIdeas(limit: number) {
  return useQuery<IdeaListItem[], ApiError>({
    queryKey: ideaKeys.recent(limit),
    queryFn: async () => (await fetchIdeas({ status: "active", q: "", limit, offset: 0 })).data,
  });
}

export function useIdea(id: string) {
  return useQuery<Idea, ApiError>({ queryKey: ideaKeys.detail(id), queryFn: () => fetchIdea(id) });
}

/** Several ideas at once (e.g. the ones a generation created or updated). */
export function useIdeasByIds(ids: string[]) {
  return useQueries({
    queries: ids.map((id) => ({
      queryKey: ideaKeys.detail(id),
      queryFn: () => fetchIdea(id),
    })),
  });
}

export function useIdeaVersion(id: string, version: number | undefined) {
  return useQuery<IdeaVersion, ApiError>({
    queryKey: ideaKeys.version(id, version ?? 0),
    queryFn: () => fetchIdeaVersion(id, version ?? 0),
    enabled: version !== undefined && version >= 1,
    staleTime: Infinity, // versions are immutable
  });
}

/** Triage with an optimistic update; the previous status is restored on failure. */
export function useUpdateIdeaStatus(id: string) {
  const client = useQueryClient();
  return useMutation<void, ApiError, IdeaStatus, { previous: Idea | undefined }>({
    mutationFn: (status) => updateIdeaStatus(id, status),
    onMutate: async (status) => {
      await client.cancelQueries({ queryKey: ideaKeys.detail(id), exact: true });
      const previous = client.getQueryData<Idea>(ideaKeys.detail(id));
      if (previous) client.setQueryData<Idea>(ideaKeys.detail(id), { ...previous, status });
      return { previous };
    },
    onError: (_error, _status, context) => {
      if (context?.previous) client.setQueryData(ideaKeys.detail(id), context.previous);
    },
    onSettled: async () => {
      await client.invalidateQueries({ queryKey: ideaKeys.lists() });
      await client.invalidateQueries({ queryKey: ideaKeys.detail(id), exact: true });
    },
  });
}

export function useEditIdea(id: string) {
  const client = useQueryClient();
  return useMutation<Idea, ApiError, EditIdeaRequest>({
    mutationFn: (body) => editIdea(id, body),
    onSuccess: async (saved) => {
      client.setQueryData(ideaKeys.detail(id), saved);
      await client.invalidateQueries({ queryKey: ideaKeys.lists() });
    },
  });
}

/** Queues a generation; while one runs the API answers with the same job. */
export function useGenerateIdeas() {
  return useMutation<string, ApiError, GenerateIdeasRequest>({ mutationFn: generateIdeas });
}

export function useDeepenIdea(id: string) {
  return useMutation<string, ApiError, DeepenIdeaRequest>({
    mutationFn: (body) => deepenIdea(id, body),
  });
}

/** Failed reads (each after the query's own retries) before a job is given up on. */
const JOB_READ_FAILURES = 5;

/**
 * Whether a job can no longer be followed: a client error (gone, not ours) will
 * not fix itself; network and server errors get a few more polls first.
 */
export function isJobUnreadable(error: ApiError | null, failures: number): boolean {
  if (!error) return false;
  return (error.status >= 400 && error.status < 500) || failures >= JOB_READ_FAILURES;
}

/** Polls a job until it reaches a final status or cannot be read any more. */
export function useIdeaJobQuery(jobId: string | null) {
  return useQuery<IdeaJob, ApiError>({
    queryKey: ideaKeys.job(jobId ?? ""),
    queryFn: () => fetchJob(jobId ?? ""),
    enabled: jobId !== null,
    staleTime: 0,
    gcTime: 60_000,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      if (status !== undefined && status !== "pending" && status !== "running") return false;
      return isJobUnreadable(query.state.error, query.state.errorUpdateCount) ? false : 2000;
    },
  });
}

/** Refreshes everything about ideas, after a generation or a deepen finished. */
export function useInvalidateIdeas() {
  const client = useQueryClient();
  return () => client.invalidateQueries({ queryKey: ideaKeys.all });
}
