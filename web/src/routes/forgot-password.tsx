import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { data, Link, redirect, useActionData, useNavigation, useSubmit } from "react-router";

import { FormField } from "@/components/common/form-field";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { fieldErrors, forgotSchema, type ForgotValues } from "@/features/auth/schemas/auth.schema";
import type { AuthActionResult } from "@/features/auth/types";
import { apiFetch, ApiCallError, setupStatus } from "@/lib/.server/api.server";
import { isTrustedMutation } from "@/lib/.server/security.server";
import { getSession } from "@/lib/.server/session.server";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/forgot-password";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "forgotPassword");
}

export async function loader({ request }: Route.LoaderArgs) {
  if (getSession(request)) throw redirect("/");
  if (!(await setupStatus(request)).passwordResetEnabled) throw redirect("/login");
  return null;
}

export async function action({ request }: Route.ActionArgs) {
  if (!isTrustedMutation(request)) return data({ formError: "FORBIDDEN_ORIGIN" }, { status: 403 });
  const parsed = forgotSchema.safeParse(Object.fromEntries(await request.formData()));
  if (!parsed.success) return data({ fields: fieldErrors(parsed.error) }, { status: 400 });
  try {
    // The API answers 202 whether or not the account exists; so does this page.
    await apiFetch("/password-resets/request", {
      method: "POST",
      body: { email: parsed.data.email },
      request,
    });
    return data({ done: true });
  } catch (err) {
    if (err instanceof ApiCallError)
      return data({ formError: err.body.code, fields: err.body.fields }, { status: err.status });
    throw err;
  }
}

export default function ForgotPassword() {
  const { t } = useTranslation(["auth", "common"]);
  const result = useActionData<AuthActionResult>();
  const submit = useSubmit();
  const submitting = useNavigation().state === "submitting";
  const form = useForm<ForgotValues>({
    resolver: zodResolver(forgotSchema),
    defaultValues: { email: "" },
  });
  const { errors } = form.formState;

  useEffect(() => {
    for (const [field, tag] of Object.entries(result?.fields ?? {}))
      form.setError(field as keyof ForgotValues, { message: tag });
  }, [result, form]);

  const back = (
    <Link to="/login" className="text-sm text-signal">
      {t("forgotPassword.back")}
    </Link>
  );

  if (result?.done) {
    return (
      <div className="flex flex-col gap-4">
        <h2 className="font-serif text-3xl font-medium">{t("forgotPassword.title")}</h2>
        <Alert>
          <AlertDescription>{t("forgotPassword.sent")}</AlertDescription>
        </Alert>
        {back}
      </div>
    );
  }

  return (
    <form
      onSubmit={form.handleSubmit((v) => submit(v, { method: "post" }))}
      noValidate
      className="flex flex-col gap-6"
    >
      <div className="flex flex-col gap-2">
        <h2 className="font-serif text-4xl font-medium">{t("forgotPassword.title")}</h2>
        <p className="text-muted-foreground">{t("forgotPassword.subtitle")}</p>
      </div>
      {result?.formError && (
        <Alert variant="destructive">
          <AlertDescription>
            {t(`forgotPassword.${result.formError}`, {
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
      <Button type="submit" size="lg" className="h-11 text-base" disabled={submitting}>
        {submitting ? t("forgotPassword.submitting") : t("forgotPassword.submit")}
      </Button>
      {back}
    </form>
  );
}
