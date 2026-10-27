import { isAxiosError } from "axios";

/** Every failed API call becomes this shape. The UI decides by code/status. */
export type ApiError = {
  status: number; // 0 = network error
  code: string;
  message: string;
  fields?: Record<string, string>;
};

type ErrorBody = { error?: { code?: unknown; message?: unknown; fields?: unknown } };

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null;
}

export function toApiError(err: unknown): ApiError {
  if (isAxiosError(err)) {
    if (!err.response) return { status: 0, code: "NETWORK_ERROR", message: err.message };
    const body = err.response.data as ErrorBody | undefined;
    const e = isRecord(body) && isRecord(body.error) ? body.error : undefined;
    const fields = e && isRecord(e.fields) ? (e.fields as Record<string, string>) : undefined;
    return {
      status: err.response.status,
      code: typeof e?.code === "string" ? e.code : "UNKNOWN_ERROR",
      message: typeof e?.message === "string" ? e.message : err.message,
      fields,
    };
  }
  return {
    status: 0,
    code: "UNKNOWN_ERROR",
    message: err instanceof Error ? err.message : String(err),
  };
}

export function isApiError(err: unknown): err is ApiError {
  return isRecord(err) && typeof err.status === "number" && typeof err.code === "string";
}
