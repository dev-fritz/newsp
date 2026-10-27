import { data } from "react-router";

import { passwordFieldErrors, passwordSchema, ProfileScreen } from "@/features/profile";
import type { PasswordActionResult } from "@/features/profile";
import { ApiCallError, apiFetch, toSession } from "@/lib/.server/api.server";
import { requireSession } from "@/lib/.server/auth.server";
import { isTrustedMutation } from "@/lib/.server/security.server";
import { commitSession } from "@/lib/.server/session.server";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/profile";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "profile");
}

type PasswordResponse = Parameters<typeof toSession>[0];

/** Changes the password server side: the new session's tokens go to the cookie only. */
export async function action({ request }: Route.ActionArgs) {
  if (!isTrustedMutation(request)) {
    return data<PasswordActionResult>({ formError: "FORBIDDEN_ORIGIN" }, { status: 403 });
  }
  const { session, setCookie } = await requireSession(request);
  const parsed = passwordSchema.safeParse(Object.fromEntries(await request.formData()));
  if (!parsed.success) {
    return data<PasswordActionResult>(
      { fields: passwordFieldErrors(parsed.error) },
      { status: 400, headers: setCookie ? { "Set-Cookie": setCookie } : undefined },
    );
  }
  try {
    const result = await apiFetch<PasswordResponse>("/me/password", {
      method: "PUT",
      token: session.accessToken,
      body: parsed.data,
      request,
    });
    return data<PasswordActionResult>(
      { done: true },
      { headers: { "Set-Cookie": commitSession(toSession(result)) } },
    );
  } catch (err) {
    if (!(err instanceof ApiCallError)) throw err;
    const wrongCurrent = err.body.fields?.current_password === "invalid";
    return data<PasswordActionResult>(
      {
        formError: wrongCurrent ? "WRONG_CURRENT_PASSWORD" : err.body.code,
        fields: err.body.fields,
      },
      { status: err.status, headers: setCookie ? { "Set-Cookie": setCookie } : undefined },
    );
  }
}

export default function Profile() {
  return <ProfileScreen />;
}
