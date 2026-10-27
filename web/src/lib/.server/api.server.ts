import { serverEnv } from "@/config/env.server";
import type { SessionData, SessionUser } from "@/lib/.server/session.server";

/** Error body of the API: { error: { code, message, fields } }. */
export type ApiErrorBody = { code: string; message: string; fields?: Record<string, string> };

export class ApiCallError extends Error {
  constructor(
    readonly status: number,
    readonly body: ApiErrorBody,
  ) {
    super(body.message);
  }
}

type SessionResponse = {
  access_token: string;
  access_expires_at: string;
  refresh_token: string;
  refresh_expires_at: string;
  user: Omit<SessionUser, "emailVerified"> & { email_verified?: boolean } & Record<string, unknown>;
};

/** Converts the API's session payload into what the cookie stores. */
export function toSession(res: SessionResponse): SessionData {
  const { id, name, email, role, locale, permissions } = res.user;
  const emailVerified = res.user.email_verified;
  return {
    accessToken: res.access_token,
    accessExpiresAt: res.access_expires_at,
    refreshToken: res.refresh_token,
    refreshExpiresAt: res.refresh_expires_at,
    user: { id, name, email, role, locale, permissions, emailVerified: emailVerified !== false },
  };
}

/** Headers that tell the API who the real client is (it trusts this server). */
export function forwardedHeaders(request: Request): Record<string, string> {
  const forwarded = request.headers.get("X-Forwarded-For");
  return forwarded ? { "X-Forwarded-For": forwarded } : {};
}

/** Server-to-server JSON call to the API. Throws ApiCallError on error bodies. */
export async function apiFetch<T>(
  path: string,
  init: { method?: string; body?: unknown; token?: string; request?: Request } = {},
): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (init.body !== undefined) headers["Content-Type"] = "application/json";
  if (init.token) headers.Authorization = `Bearer ${init.token}`;
  if (init.request) Object.assign(headers, forwardedHeaders(init.request));

  let res: Response;
  try {
    res = await fetch(`${serverEnv().API_URL}/api/v1${path}`, {
      method: init.method ?? "GET",
      headers,
      body: init.body === undefined ? undefined : JSON.stringify(init.body),
      signal: AbortSignal.timeout(30_000),
    });
  } catch {
    throw new ApiCallError(503, { code: "API_UNREACHABLE", message: "the API is unreachable" });
  }
  if (res.status === 204) return undefined as T;
  const json = (await res.json().catch(() => null)) as { data?: T; error?: ApiErrorBody } | null;
  if (!res.ok) {
    throw new ApiCallError(
      res.status,
      json?.error ?? { code: "UNKNOWN_ERROR", message: res.statusText },
    );
  }
  return json?.data as T;
}

export async function loginWithPassword(
  request: Request,
  email: string,
  password: string,
): Promise<SessionData> {
  return toSession(
    await apiFetch<SessionResponse>("/auth/login", {
      method: "POST",
      body: { email, password },
      request,
    }),
  );
}

export async function setupAdmin(
  request: Request,
  input: { name: string; email: string; password: string },
): Promise<SessionData> {
  return toSession(
    await apiFetch<SessionResponse>("/setup", { method: "POST", body: input, request }),
  );
}

export async function acceptInvite(
  request: Request,
  input: { token: string; name: string; email: string; password: string },
): Promise<SessionData> {
  return toSession(
    await apiFetch<SessionResponse>("/invites/accept", { method: "POST", body: input, request }),
  );
}

export async function signup(
  request: Request,
  input: { name: string; email: string; password: string; locale?: "pt-BR" | "en" },
): Promise<SessionData> {
  return toSession(
    await apiFetch<SessionResponse>("/auth/signup", { method: "POST", body: input, request }),
  );
}

export async function rotateSession(refreshToken: string, request?: Request): Promise<SessionData> {
  return toSession(
    await apiFetch<SessionResponse>("/auth/refresh", {
      method: "POST",
      body: { refresh_token: refreshToken },
      request,
    }),
  );
}

export type SetupStatus = {
  setupRequired: boolean;
  signupEnabled: boolean;
  passwordResetEnabled: boolean;
};

/** What the public pages need to know about the instance. */
export async function setupStatus(request: Request): Promise<SetupStatus> {
  const res = await apiFetch<{
    setup_required?: boolean;
    signup_enabled?: boolean;
    password_reset_enabled?: boolean;
  }>("/setup/status", { request });
  return {
    setupRequired: res.setup_required === true,
    signupEnabled: res.signup_enabled === true,
    passwordResetEnabled: res.password_reset_enabled === true,
  };
}

export async function setupRequired(request: Request): Promise<boolean> {
  return (await setupStatus(request)).setupRequired;
}

/**
 * Re-reads the e-mail verification of an unverified session's user. The link
 * may have been opened in another browser or for another account, so only the
 * API can say whether this session's user is verified now. Returns the updated
 * session, or null when nothing changed.
 */
export async function syncEmailVerified(session: SessionData): Promise<SessionData | null> {
  if (session.user.emailVerified !== false) return null;
  const me = await apiFetch<{ email_verified?: boolean }>("/me", { token: session.accessToken });
  if (me.email_verified !== true) return null;
  return { ...session, user: { ...session.user, emailVerified: true } };
}
