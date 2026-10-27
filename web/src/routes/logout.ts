import { redirect } from "react-router";

import { apiFetch } from "@/lib/.server/api.server";
import { isTrustedMutation } from "@/lib/.server/security.server";
import { destroySession, getSession } from "@/lib/.server/session.server";

import type { Route } from "./+types/logout";

export async function action({ request }: Route.ActionArgs) {
  if (!isTrustedMutation(request)) return redirect("/");
  const session = getSession(request);
  if (session) {
    // Best effort: the cookie is dropped even if the API cannot be reached.
    await apiFetch("/auth/logout", {
      method: "POST",
      body: { refresh_token: session.refreshToken },
      request,
    }).catch(() => undefined);
  }
  return redirect("/login", { headers: { "Set-Cookie": destroySession() } });
}

export function loader() {
  return redirect("/");
}
