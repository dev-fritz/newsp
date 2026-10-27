import {
  type InfiniteData,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";

import type { ApiError } from "@/lib/api-error";

import { isActive, type Report, type ReportKind } from "../types";
import {
  createReport,
  deleteReport,
  getReport,
  listReports,
  type CreateReportInput,
  type ReportsPage,
} from "./reports-api";

const PAGE_SIZE = 20;
const POLL_MS = 4_000;
/** How many recent reports `useLatestReport` inspects to find a finished one. */
const LATEST_WINDOW = 20;

export const reportKeys = {
  all: ["reports"] as const,
  lists: () => [...reportKeys.all, "list"] as const,
  list: (kind: ReportKind | undefined) => [...reportKeys.lists(), kind ?? "all"] as const,
  details: () => [...reportKeys.all, "detail"] as const,
  detail: (id: string) => [...reportKeys.details(), id] as const,
  latest: () => [...reportKeys.all, "latest"] as const,
};

function pagesHaveActive(pages: ReportsPage[] | undefined): boolean {
  return pages?.some((page) => page.data.some(isActive)) ?? false;
}

/** Paginated report list; polls while any report is pending or running. */
export function useReportsInfinite(kind: ReportKind | undefined) {
  const queryClient = useQueryClient();
  const key = reportKeys.list(kind);
  return useInfiniteQuery<
    ReportsPage,
    ApiError,
    InfiniteData<ReportsPage, number>,
    ReturnType<typeof reportKeys.list>,
    number
  >({
    queryKey: key,
    initialPageParam: 0,
    queryFn: async ({ pageParam }) => {
      const page = await listReports({ kind, limit: PAGE_SIZE, offset: pageParam });
      if (pageParam === 0) {
        // A report just finished: whatever was cached about "the latest report" is stale.
        const previous = queryClient.getQueryData<InfiniteData<ReportsPage, number>>(key);
        if (pagesHaveActive(previous?.pages) && !pagesHaveActive([page])) {
          void queryClient.invalidateQueries({ queryKey: reportKeys.latest() });
          void queryClient.invalidateQueries({ queryKey: reportKeys.details() });
        }
      }
      return page;
    },
    getNextPageParam: (last) => {
      const next = last.meta.offset + last.data.length;
      return last.data.length > 0 && next < last.meta.total ? next : undefined;
    },
    refetchInterval: (query) => (pagesHaveActive(query.state.data?.pages) ? POLL_MS : false),
  });
}

/** One report with its content and citations; polls while it is being generated. */
export function useReport(id: string) {
  return useQuery<Report, ApiError>({
    queryKey: reportKeys.detail(id),
    queryFn: () => getReport(id),
    refetchInterval: (query) => (isActive(query.state.data) ? POLL_MS : false),
  });
}

/** The latest finished (`done`) report, or null when there is none yet. */
export function useLatestReport() {
  return useQuery<Report | null, ApiError>({
    queryKey: reportKeys.latest(),
    queryFn: async () => {
      const page = await listReports({ limit: LATEST_WINDOW, offset: 0 });
      const latest = page.data.find((report) => report.status === "done");
      return latest?.id ? getReport(latest.id) : null;
    },
  });
}

/** Queues a manual report; the list then shows it as pending and polls. */
export function useCreateReport() {
  const queryClient = useQueryClient();
  return useMutation<unknown, ApiError, CreateReportInput>({
    mutationFn: createReport,
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: reportKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: reportKeys.latest() }),
      ]);
    },
  });
}

/** Permanently deletes a finished report and drops it from every cached view. */
export function useDeleteReport() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, string>({
    mutationFn: deleteReport,
    onSuccess: (_data, id) => {
      queryClient.removeQueries({ queryKey: reportKeys.detail(id) });
      void queryClient.invalidateQueries({ queryKey: reportKeys.lists() });
      void queryClient.invalidateQueries({ queryKey: reportKeys.latest() });
    },
  });
}
