import { isApiError, type ApiError } from "@/lib/api-error";

/** The ApiError of a failed query (null when there is none). */
export function errorOf(error: unknown): ApiError | null {
  if (isApiError(error)) return error;
  return error ? { status: 0, code: "UNKNOWN_ERROR", message: "" } : null;
}

export const FORBIDDEN: ApiError = { status: 403, code: "FORBIDDEN", message: "" };
