import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { data, Link, useActionData, useNavigation, useSubmit } from "react-router";

import { FormField } from "@/components/common/form-field";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { fieldErrors, resetSchema, type ResetValues } from "@/features/auth/schemas/auth.schema";
import type { AuthActionResult } from "@/features/auth/types";
import { apiFetch, ApiCallError } from "@/lib/.server/api.server";
import { isTrustedMutation } from "@/lib/.server/security.server";
import { destroySession } from "@/lib/.server/session.server";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/reset-password";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "resetPassword");
}

export async function loader({ request, params }: Route.LoaderArgs) {
  try {
    await apiFetch("/password-resets/inspect", {
      method: "POST",
      body: { token: params.token },
      request,
    });
    return { valid: true };
  } catch (err) {
    if (err instanceof ApiCallError && err.status === 404) return { valid: false };
    throw err;
  }
}

export async function action({ request, params }: Route.ActionArgs) {
  if (!isTrustedMutation(request)) return data({ formError: "FORBIDDEN_ORIGIN" }, { status: 403 });
  const parsed = resetSchema.safeParse(Object.fromEntries(await request.formData()));
  if (!parsed.success) return data({ fields: fieldErrors(parsed.error) }, { status: 400 });
  try {
    await apiFetch("/password-resets/confirm", {
      method: "POST",
      body: { token: params.token, password: parsed.data.password },
      request,
    });
    // Every session of the user ends; drop this browser's too.
    return data({ done: true }, { headers: { "Set-Cookie": destroySession() } });
  } catch (err) {
    if (err instanceof ApiCallError)
      return data({ formError: err.body.code, fields: err.body.fields }, { status: err.status });
    throw err;
  }
}

export default function ResetPassword({ loaderData }: Route.ComponentProps) {
  const { t } = useTranslation(["auth", "common"]);
  const result = useActionData<AuthActionResult>();
  const submit = useSubmit();
  const submitting = useNavigation().state === "submitting";
  const form = useForm<ResetValues>({
    resolver: zodResolver(resetSchema),
    defaultValues: { password: "", confirm: "" },
  });
  const { errors } = form.formState;

  useEffect(() => {
    for (const [field, tag] of Object.entries(result?.fields ?? {}))
      form.setError(field as keyof ResetValues, { message: tag });
  }, [result, form]);

  if (!loaderData.valid || result?.done || result?.formError === "INVALID_LINK") {
    return (
      <div className="flex flex-col gap-4">
        <h2 className="font-serif text-3xl font-medium">{t("reset.title")}</h2>
        <Alert variant={result?.done ? "default" : "destructive"}>
          <AlertDescription>
            {result?.done ? (
              t("reset.done")
            ) : (
              <>
                {t("reset.INVALID_LINK")} <Link to="/forgot-password">{t("reset.requestNew")}</Link>{" "}
                {t("reset.orAskAdmin")}
              </>
            )}
          </AlertDescription>
        </Alert>
        <Link to="/login" className="text-signal">
          {t("goLogin")}
        </Link>
      </div>
    );
  }

  return (
    <form
      onSubmit={form.handleSubmit((v) => submit(v, { method: "post", replace: true }))}
      noValidate
      className="flex flex-col gap-6"
    >
      <div className="flex flex-col gap-2">
        <h2 className="font-serif text-4xl font-medium">{t("reset.title")}</h2>
        <p className="text-muted-foreground">{t("reset.subtitle")}</p>
      </div>
      {result?.formError && (
        <Alert variant="destructive">
          <AlertDescription>
            {t(`reset.${result.formError}`, {
              defaultValue: t(`common:errors.${result.formError}`, {
                defaultValue: t("common:errors.UNKNOWN_ERROR"),
              }),
            })}
          </AlertDescription>
        </Alert>
      )}
      <FormField
        label={t("setup.password")}
        type="password"
        autoComplete="new-password"
        hint={t("setup.passwordHint")}
        error={errors.password?.message}
        {...form.register("password")}
      />
      <FormField
        label={t("setup.confirm")}
        type="password"
        autoComplete="new-password"
        error={errors.confirm?.message}
        {...form.register("confirm")}
      />
      <Button type="submit" size="lg" className="h-11 text-base" disabled={submitting}>
        {t("reset.submit")}
      </Button>
    </form>
  );
}
