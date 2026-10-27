import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import {
  data,
  Link,
  redirect,
  useActionData,
  useNavigation,
  useSearchParams,
  useSubmit,
} from "react-router";

import { FormField } from "@/components/common/form-field";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { fieldErrors, loginSchema, type LoginValues } from "@/features/auth/schemas/auth.schema";
import type { AuthActionResult } from "@/features/auth/types";
import { ApiCallError, loginWithPassword, setupStatus } from "@/lib/.server/api.server";
import { isTrustedMutation } from "@/lib/.server/security.server";
import { commitSession, getSession } from "@/lib/.server/session.server";
import { safeRedirect } from "@/lib/safe-redirect";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/login";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "login");
}

export async function loader({ request }: Route.LoaderArgs) {
  if (getSession(request)) throw redirect("/");
  const status = await setupStatus(request).catch(() => null);
  if (status?.setupRequired) throw redirect("/setup");
  return {
    signupEnabled: status?.signupEnabled ?? false,
    passwordResetEnabled: status?.passwordResetEnabled ?? false,
  };
}

export async function action({ request }: Route.ActionArgs) {
  if (!isTrustedMutation(request)) return data({ formError: "FORBIDDEN_ORIGIN" }, { status: 403 });
  const parsed = loginSchema.safeParse(Object.fromEntries(await request.formData()));
  if (!parsed.success) return data({ fields: fieldErrors(parsed.error) }, { status: 400 });
  try {
    const session = await loginWithPassword(request, parsed.data.email, parsed.data.password);
    return redirect(safeRedirect(parsed.data.redirect), {
      headers: { "Set-Cookie": commitSession(session) },
    });
  } catch (err) {
    if (err instanceof ApiCallError)
      return data({ formError: err.body.code, fields: err.body.fields }, { status: err.status });
    throw err;
  }
}

export default function Login({ loaderData }: Route.ComponentProps) {
  const { t } = useTranslation(["auth", "common"]);
  const result = useActionData<AuthActionResult>();
  const submit = useSubmit();
  const navigation = useNavigation();
  const [params] = useSearchParams();
  const form = useForm<LoginValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
  });
  const { errors } = form.formState;

  useEffect(() => {
    for (const [field, tag] of Object.entries(result?.fields ?? {})) {
      form.setError(field as keyof LoginValues, { message: tag });
    }
  }, [result, form]);

  const submitting = navigation.state === "submitting";
  const onSubmit = form.handleSubmit((values) =>
    submit({ ...values, redirect: params.get("redirect") ?? "/" }, { method: "post" }),
  );

  return (
    <form
      onSubmit={onSubmit}
      noValidate
      className="flex flex-col gap-6"
      aria-labelledby="login-title"
    >
      <div className="flex flex-col gap-2">
        <h2 id="login-title" className="font-serif text-4xl font-medium">
          {t("login.title")}
        </h2>
        <p className="text-muted-foreground">{t("login.subtitle")}</p>
      </div>
      {result?.formError && (
        <Alert variant="destructive">
          <AlertDescription>
            {t(`login.${result.formError}`, {
              defaultValue: t(`common:errors.${result.formError}`, {
                defaultValue: t("common:errors.UNKNOWN_ERROR"),
              }),
            })}
          </AlertDescription>
        </Alert>
      )}
      <FormField
        label={t("login.email")}
        type="email"
        autoComplete="email"
        error={errors.email?.message}
        {...form.register("email")}
      />
      <FormField
        label={t("login.password")}
        type="password"
        autoComplete="current-password"
        error={errors.password?.message}
        {...form.register("password")}
      />
      <Button type="submit" size="lg" className="h-11 text-base" disabled={submitting}>
        {submitting ? t("login.submitting") : t("login.submit")}
      </Button>
      <div className="flex flex-col gap-2 text-sm leading-relaxed text-subtle-foreground">
        {loaderData.passwordResetEnabled ? (
          <Link to="/forgot-password" className="text-signal">
            {t("login.forgotLink")}
          </Link>
        ) : (
          <p>{t("login.forgot")}</p>
        )}
        {loaderData.signupEnabled && (
          <p>
            {t("login.noAccount")}{" "}
            <Link to="/signup" className="text-signal">
              {t("login.createAccount")}
            </Link>
          </p>
        )}
      </div>
    </form>
  );
}
