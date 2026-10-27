import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { data, redirect, useActionData, useNavigation, useSubmit } from "react-router";

import { FormField } from "@/components/common/form-field";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { SetupSteps } from "@/features/auth/components/setup-steps";
import {
  accountSchema,
  fieldErrors,
  type AccountValues,
} from "@/features/auth/schemas/auth.schema";
import type { AuthActionResult } from "@/features/auth/types";
import { ApiCallError, setupAdmin, setupRequired } from "@/lib/.server/api.server";
import { isTrustedMutation } from "@/lib/.server/security.server";
import { commitSession } from "@/lib/.server/session.server";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/setup";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "setup");
}

export async function loader({ request }: Route.LoaderArgs) {
  if (!(await setupRequired(request))) throw redirect("/login");
  return null;
}

export async function action({ request }: Route.ActionArgs) {
  if (!isTrustedMutation(request)) return data({ formError: "FORBIDDEN_ORIGIN" }, { status: 403 });
  const parsed = accountSchema.safeParse(Object.fromEntries(await request.formData()));
  if (!parsed.success) return data({ fields: fieldErrors(parsed.error) }, { status: 400 });
  try {
    const { name, email, password } = parsed.data;
    const session = await setupAdmin(request, { name, email, password });
    return redirect("/setup/ai", { headers: { "Set-Cookie": commitSession(session) } });
  } catch (err) {
    if (err instanceof ApiCallError)
      return data({ formError: err.body.code, fields: err.body.fields }, { status: err.status });
    throw err;
  }
}

export default function Setup() {
  const { t } = useTranslation(["auth", "common"]);
  const result = useActionData<AuthActionResult>();
  const submit = useSubmit();
  const submitting = useNavigation().state === "submitting";
  const form = useForm<AccountValues>({
    resolver: zodResolver(accountSchema),
    defaultValues: { name: "", email: "", password: "", confirm: "" },
  });
  const { errors } = form.formState;

  useEffect(() => {
    for (const [field, tag] of Object.entries(result?.fields ?? {}))
      form.setError(field as keyof AccountValues, { message: tag });
  }, [result, form]);

  return (
    <form
      onSubmit={form.handleSubmit((v) => submit(v, { method: "post" }))}
      noValidate
      className="flex flex-col gap-6"
    >
      <SetupSteps current={1} />
      <div className="flex flex-col gap-2">
        <span className="kicker">{t("setup.stepOf", { current: 1, total: 3 })}</span>
        <h2 className="font-serif text-4xl leading-tight font-medium">{t("setup.title")}</h2>
        <p className="text-muted-foreground">{t("setup.subtitle")}</p>
      </div>
      {result?.formError && (
        <Alert variant="destructive">
          <AlertDescription>
            {t(`setup.${result.formError}`, {
              defaultValue: t(`common:errors.${result.formError}`, {
                defaultValue: t("common:errors.UNKNOWN_ERROR"),
              }),
            })}
          </AlertDescription>
        </Alert>
      )}
      <FormField
        label={t("setup.name")}
        autoComplete="name"
        error={errors.name?.message}
        {...form.register("name")}
      />
      <FormField
        label={t("setup.email")}
        type="email"
        autoComplete="email"
        error={errors.email?.message}
        {...form.register("email")}
      />
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
        {t("setup.submit")}
      </Button>
    </form>
  );
}
