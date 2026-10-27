import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { data, Link, redirect, useActionData, useNavigation, useSubmit } from "react-router";

import { FormField } from "@/components/common/form-field";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  accountSchema,
  fieldErrors,
  type AccountValues,
} from "@/features/auth/schemas/auth.schema";
import type { AuthActionResult } from "@/features/auth/types";
import { ApiCallError, setupStatus, signup } from "@/lib/.server/api.server";
import { isTrustedMutation } from "@/lib/.server/security.server";
import { commitSession, getSession } from "@/lib/.server/session.server";
import { isLocale } from "@/i18n";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/signup";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "signup");
}

export async function loader({ request }: Route.LoaderArgs) {
  if (getSession(request)) throw redirect("/");
  const status = await setupStatus(request);
  if (status.setupRequired) throw redirect("/setup");
  if (!status.signupEnabled) throw redirect("/login");
  return null;
}

export async function action({ request }: Route.ActionArgs) {
  if (!isTrustedMutation(request)) return data({ formError: "FORBIDDEN_ORIGIN" }, { status: 403 });
  const form = Object.fromEntries(await request.formData());
  const parsed = accountSchema.safeParse(form);
  if (!parsed.success) return data({ fields: fieldErrors(parsed.error) }, { status: 400 });
  try {
    const { name, email, password } = parsed.data;
    const locale = isLocale(form.locale) ? form.locale : undefined;
    const session = await signup(request, { name, email, password, locale });
    return redirect("/", { headers: { "Set-Cookie": commitSession(session) } });
  } catch (err) {
    if (err instanceof ApiCallError)
      return data({ formError: err.body.code, fields: err.body.fields }, { status: err.status });
    throw err;
  }
}

export default function Signup() {
  const { t, i18n } = useTranslation(["auth", "common"]);
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

  // The account starts in the language the visitor is reading right now.
  const locale = i18n.language === "en" ? "en" : "pt-BR";
  const onSubmit = form.handleSubmit((values) => submit({ ...values, locale }, { method: "post" }));

  return (
    <form
      onSubmit={onSubmit}
      noValidate
      className="flex flex-col gap-6"
      aria-labelledby="signup-title"
    >
      <div className="flex flex-col gap-2">
        <h2 id="signup-title" className="font-serif text-4xl font-medium">
          {t("signup.title")}
        </h2>
        <p className="text-muted-foreground">{t("signup.subtitle")}</p>
      </div>
      {result?.formError && (
        <Alert variant="destructive">
          <AlertDescription>
            {t(`signup.${result.formError}`, {
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
        {submitting ? t("signup.submitting") : t("signup.submit")}
      </Button>
      <p className="text-sm leading-relaxed text-subtle-foreground">
        {t("signup.hasAccount")}{" "}
        <Link to="/login" className="text-signal">
          {t("signup.signIn")}
        </Link>
      </p>
    </form>
  );
}
