import { serverEnv } from "@/config/env.server";
import { forwardedHeaders } from "@/lib/.server/api.server";
import { activeSession, refreshSession } from "@/lib/.server/auth.server";
import { isTrustedMutation } from "@/lib/.server/security.server";
import { commitSession, destroySession, type SessionData } from "@/lib/.server/session.server";

import type { Route } from "./+types/api";

const MAX_BODY_BYTES = 2 * 1024 * 1024;
const API_PREFIX = "/api/v1/";

// These endpoints answer with access and refresh tokens. The server-side
// actions call them directly; through the proxy they would hand the tokens to
// browser JavaScript.
const TOKEN_ENDPOINTS = new Set([
  "auth/login",
  "auth/refresh",
  "auth/signup",
  "invites/accept",
  "me/password",
  "setup",
]);

/**
 * Resolves the API URL for a proxied path, or null when it would leave
 * /api/v1 (the URL parser turns "\\" into "/" and resolves dot segments) or
 * reach an endpoint that returns tokens.
 */
function apiTarget(apiUrl: string, path: string, search: string): URL | null {
  const url = new URL(`${API_PREFIX}${path}${search}`, apiUrl);
  if (!url.pathname.startsWith(API_PREFIX)) return null;
  const endpoint = url.pathname.slice(API_PREFIX.length).replace(/\/+$/, "");
  if (TOKEN_ENDPOINTS.has(endpoint)) return null;
  return url;
}

function jsonError(
  status: number,
  code: string,
  message: string,
  headers: HeadersInit = {},
): Response {
  return Response.json({ error: { code, message } }, { status, headers });
}

/**
 * Same-origin proxy: the browser calls /api/<path>, this server adds the access
 * token from the encrypted session and forwards to the API's /api/v1/<path>.
 * Tokens never reach client JavaScript.
 */
async function proxy(request: Request, path: string | undefined): Promise<Response> {
  const target = path ? apiTarget(serverEnv().API_URL, path, new URL(request.url).search) : null;
  if (!target) return jsonError(404, "NOT_FOUND", "not found");
  if (!isTrustedMutation(request)) {
    return jsonError(403, "FORBIDDEN_ORIGIN", "cross-origin request refused");
  }
  const active = await activeSession(request);
  if (!active) {
    return jsonError(401, "UNAUTHENTICATED", "authentication required", {
      "Set-Cookie": destroySession(),
    });
  }

  let body: ArrayBuffer | undefined;
  if (request.method !== "GET" && request.method !== "HEAD") {
    body = await request.arrayBuffer();
    if (body.byteLength > MAX_BODY_BYTES)
      return jsonError(413, "PAYLOAD_TOO_LARGE", "request body too large");
  }
  const send = (session: SessionData) => {
    const headers: Record<string, string> = {
      Accept: "application/json",
      Authorization: `Bearer ${session.accessToken}`,
      ...forwardedHeaders(request),
    };
    const contentType = request.headers.get("Content-Type");
    if (contentType) headers["Content-Type"] = contentType;
    return fetch(target, {
      method: request.method,
      headers,
      body,
      signal: AbortSignal.timeout(60_000),
    });
  };

  let session = active.session;
  let setCookie = active.setCookie;
  let upstream: Response;
  try {
    upstream = await send(session);
    if (upstream.status === 401) {
      // The access token may have been revoked or expired in flight: one refresh, one retry.
      const next = await refreshSession(session, request);
      if (!next) {
        return jsonError(401, "UNAUTHENTICATED", "authentication required", {
          "Set-Cookie": destroySession(),
        });
      }
      session = next;
      setCookie = commitSession(next);
      upstream = await send(session);
    }
  } catch {
    return jsonError(502, "API_UNREACHABLE", "the API is unreachable");
  }

  const headers = new Headers();
  const contentType = upstream.headers.get("Content-Type");
  if (contentType) headers.set("Content-Type", contentType);
  const retryAfter = upstream.headers.get("Retry-After");
  if (retryAfter) headers.set("Retry-After", retryAfter);
  headers.set("Cache-Control", "no-store");
  if (setCookie) headers.append("Set-Cookie", setCookie);
  return new Response(upstream.body, { status: upstream.status, headers });
}

export async function loader({ request, params }: Route.LoaderArgs) {
  return proxy(request, params["*"]);
}

export async function action({ request, params }: Route.ActionArgs) {
  return proxy(request, params["*"]);
}
