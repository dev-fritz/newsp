import { serverEnv } from "@/config/env.server";

const SAFE_METHODS = new Set(["GET", "HEAD", "OPTIONS"]);

/**
 * CSRF defense for cookie-authenticated mutations: the request must come from
 * our own origin. SameSite=Lax cookies block most cross-site posts already;
 * this closes the remaining gaps (same-site subdomains, old browsers).
 */
export function isTrustedMutation(request: Request): boolean {
  if (SAFE_METHODS.has(request.method)) return true;
  const expected = new URL(serverEnv().APP_ORIGIN).origin;
  const origin = request.headers.get("Origin");
  if (origin) return origin === expected;
  const referer = request.headers.get("Referer");
  if (referer) {
    try {
      return new URL(referer).origin === expected;
    } catch {
      return false;
    }
  }
  return false;
}
