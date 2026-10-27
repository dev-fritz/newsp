import { redirect } from "react-router";

import { ApiCallError, rotateSession } from "@/lib/.server/api.server";
import {
  commitSession,
  destroySession,
  getSession,
  type SessionData,
} from "@/lib/.server/session.server";

/** Refresh slightly before expiry so a request never carries a dead token. */
const REFRESH_MARGIN_MS = 30_000;
/**
 * A rotated refresh token stays answerable for this long: parallel requests
 * that still carry the old cookie get the same new session instead of
 * presenting a used token, which the API treats as theft.
 */
const ROTATION_GRACE_MS = 30_000;

type Rotation = { promise: Promise<SessionData | null>; settledAt?: number };
const rotations = new Map<string, Rotation>();

/** Single-flight refresh: concurrent callers with the same token share one call. */
export function refreshSession(
  session: SessionData,
  request?: Request,
): Promise<SessionData | null> {
  const now = Date.now();
  for (const [token, r] of rotations) {
    if (r.settledAt && now - r.settledAt > ROTATION_GRACE_MS) rotations.delete(token);
  }
  const existing = rotations.get(session.refreshToken);
  if (existing) return existing.promise;

  const rotation: Rotation = {
    promise: rotateSession(session.refreshToken, request).then(
      (next) => next,
      (err: unknown) => {
        if (err instanceof ApiCallError && err.status < 500) return null; // revoked or expired
        rotations.delete(session.refreshToken); // transient: let the next request retry
        throw err;
      },
    ),
  };
  rotation.promise
    .finally(() => {
      rotation.settledAt = Date.now();
    })
    .catch(() => undefined);
  rotations.set(session.refreshToken, rotation);
  return rotation.promise;
}

export function needsRefresh(session: SessionData): boolean {
  return new Date(session.accessExpiresAt).getTime() - Date.now() < REFRESH_MARGIN_MS;
}

export type ActiveSession = { session: SessionData; setCookie?: string };

/** Returns a usable session, refreshing it when needed; null when signed out. */
export async function activeSession(request: Request): Promise<ActiveSession | null> {
  const session = getSession(request);
  if (!session) return null;
  if (!needsRefresh(session)) return { session };
  const next = await refreshSession(session, request);
  return next ? { session: next, setCookie: commitSession(next) } : null;
}

function loginRedirect(request: Request): Response {
  const url = new URL(request.url);
  const params = new URLSearchParams({ redirect: url.pathname + url.search });
  return redirect(`/login?${params}`, { headers: { "Set-Cookie": destroySession() } });
}

/** For protected loaders and actions: the session or a redirect to login. */
export async function requireSession(request: Request): Promise<ActiveSession> {
  const active = await activeSession(request);
  if (!active) throw loginRedirect(request);
  return active;
}

/** Test hook: forget cached rotations. */
export function resetRotationsForTests(): void {
  rotations.clear();
}
