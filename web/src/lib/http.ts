import { create } from "axios";

import { toApiError } from "@/lib/api-error";

/**
 * The only HTTP client of the browser. It talks to this app's own /api proxy,
 * which adds credentials server side; errors are normalized to ApiError.
 */
export const http = create({
  baseURL: "/api",
  timeout: 30_000,
  withCredentials: true,
  headers: { Accept: "application/json" },
});

http.interceptors.response.use(
  (res) => res,
  (err: unknown) => {
    const apiError = toApiError(err);
    if (apiError.status === 401 && typeof window !== "undefined") {
      // The session is gone (revoked, expired): start over at the login page.
      const here = window.location.pathname + window.location.search;
      window.location.assign(`/login?redirect=${encodeURIComponent(here)}`);
    }
    return Promise.reject(apiError);
  },
);

/** Unwraps the API's { data } envelope. */
export async function getData<T>(url: string, params?: Record<string, unknown>): Promise<T> {
  const { data } = await http.get<{ data: T }>(url, { params });
  return data.data;
}

/** Unwraps a paginated { data, meta } response. */
export async function getPage<T>(
  url: string,
  params?: Record<string, unknown>,
): Promise<{ data: T[]; meta: PageMeta }> {
  const { data } = await http.get<{ data: T[]; meta: PageMeta }>(url, { params });
  return data;
}

export type PageMeta = { total: number; limit: number; offset: number };

export async function send<T>(
  method: "post" | "put" | "patch" | "delete",
  url: string,
  body?: unknown,
): Promise<T> {
  const { data } = await http.request<{ data: T } | undefined>({ method, url, data: body });
  return (data?.data ?? undefined) as T;
}
