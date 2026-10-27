import { useTranslation } from "react-i18next";
import { data, Form, Link, useActionData, useNavigation } from "react-router";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import type { AuthActionResult } from "@/features/auth/types";
import { apiFetch, ApiCallError, syncEmailVerified } from "@/lib/.server/api.server";
import { activeSession } from "@/lib/.server/auth.server";
import { isTrustedMutation } from "@/lib/.server/security.server";
import { commitSession, getSession } from "@/lib/.server/session.server";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/verify-email";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "verifyEmail");
}

// Nothing is verified on GET: mail scanners and link previews open every link
// in a message. The token is only spent by the POST below.
export function loader({ request }: Route.LoaderArgs) {
  return { signedIn: getSession(request) !== null };
}

export async function action({ request, params }: Route.ActionArgs) {
  if (!isTrustedMutation(request)) return data({ formError: "FORBIDDEN_ORIGIN" }, { status: 403 });
  try {
    await apiFetch("/auth/verify-email", {
      method: "POST",
      body: { token: params.token },
      request,
    });
  } catch (err) {
    if (err instanceof ApiCallError)
      return data({ formError: err.body.code }, { status: err.status });
    throw err;
  }
  // Hide the banner right away when this browser's session is the verified one.
  const active = await activeSession(request).catch(() => null);
  const synced = active ? await syncEmailVerified(active.session).catch(() => null) : null;
  const cookie = synced ? commitSession(synced) : active?.setCookie;
  return data({ done: true }, cookie ? { headers: { "Set-Cookie": cookie } } : undefined);
}

export default function VerifyEmail({ loaderData }: Route.ComponentProps) {
  const { t } = useTranslation(["auth", "common"]);
  const result = useActionData<AuthActionResult>();
  const submitting = useNavigation().state === "submitting";

  if (result?.done) {
    return (
      <div className="flex flex-col gap-4">
        <h2 className="font-serif text-3xl font-medium">{t("verifyEmail.title")}</h2>
        <Alert>
          <AlertDescription>{t("verifyEmail.done")}</AlertDescription>
        </Alert>
        <Button asChild size="lg" className="h-11 text-base">
          {loaderData.signedIn ? (
            <Link to="/">{t("verifyEmail.goApp")}</Link>
          ) : (
            <Link to="/login">{t("goLogin")}</Link>
          )}
        </Button>
      </div>
    );
  }

  if (result?.formError === "INVALID_LINK") {
    return (
      <div className="flex flex-col gap-4">
        <h2 className="font-serif text-3xl font-medium">{t("verifyEmail.title")}</h2>
        <Alert variant="destructive">
          <AlertDescription>{t("verifyEmail.INVALID_LINK")}</AlertDescription>
        </Alert>
        <Link to={loaderData.signedIn ? "/" : "/login"} className="text-signal">
          {loaderData.signedIn ? t("verifyEmail.goApp") : t("goLogin")}
        </Link>
      </div>
    );
  }

  return (
    <Form method="post" replace className="flex flex-col gap-6">
      <div className="flex flex-col gap-2">
        <h2 className="font-serif text-4xl font-medium">{t("verifyEmail.title")}</h2>
        <p className="text-muted-foreground">{t("verifyEmail.subtitle")}</p>
      </div>
      {result?.formError && (
        <Alert variant="destructive">
          <AlertDescription>
            {t(`common:errors.${result.formError}`, {
              defaultValue: t("common:errors.UNKNOWN_ERROR"),
            })}
          </AlertDescription>
        </Alert>
      )}
      <Button type="submit" size="lg" className="h-11 text-base" disabled={submitting}>
        {submitting ? t("verifyEmail.submitting") : t("verifyEmail.submit")}
      </Button>
    </Form>
  );
}
