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
import { acceptInvite, apiFetch, ApiCallError } from "@/lib/.server/api.server";
import { isTrustedMutation } from "@/lib/.server/security.server";
import { commitSession } from "@/lib/.server/session.server";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/invite";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "invite");
}

export async function loader({ request, params }: Route.LoaderArgs) {
  try {
    const info = await apiFetch<{
      role: "admin" | "member";
      expires_at: string;
      email?: string | null;
    }>("/invites/inspect", {
      method: "POST",
      body: { token: params.token },
      request,
    });
    return {
      valid: true as const,
      role: info.role,
      expiresAt: info.expires_at,
      // Set when the invite is bound to an address; the API rejects any other.
      email: info.email ?? null,
    };
  } catch (err) {
    if (err instanceof ApiCallError && err.status === 404) return { valid: false as const };
    throw err;
  }
}

export async function action({ request, params }: Route.ActionArgs) {
  if (!isTrustedMutation(request)) return data({ formError: "FORBIDDEN_ORIGIN" }, { status: 403 });
  const parsed = accountSchema.safeParse(Object.fromEntries(await request.formData()));
  if (!parsed.success) return data({ fields: fieldErrors(parsed.error) }, { status: 400 });
  try {
    const { name, email, password } = parsed.data;
    const session = await acceptInvite(request, { token: params.token, name, email, password });
    // replace: the one-time token must not stay in the history.
    return redirect("/", { headers: { "Set-Cookie": commitSession(session) } });
  } catch (err) {
    if (err instanceof ApiCallError)
      return data({ formError: err.body.code, fields: err.body.fields }, { status: err.status });
    throw err;
  }
}

export default function Invite({ loaderData }: Route.ComponentProps) {
  const { t, i18n } = useTranslation(["auth", "common"]);
  const result = useActionData<AuthActionResult>();
  const submit = useSubmit();
  const submitting = useNavigation().state === "submitting";
  const form = useForm<AccountValues>({
    resolver: zodResolver(accountSchema),
    defaultValues: {
      name: "",
      email: loaderData.valid ? (loaderData.email ?? "") : "",
      password: "",
      confirm: "",
    },
  });
  const { errors } = form.formState;

  useEffect(() => {
    for (const [field, tag] of Object.entries(result?.fields ?? {}))
      form.setError(field as keyof AccountValues, { message: tag });
  }, [result, form]);

  if (!loaderData.valid) {
    return (
      <div className="flex flex-col gap-4">
        <h2 className="font-serif text-3xl font-medium">{t("invite.title")}</h2>
        <Alert variant="destructive">
          <AlertDescription>{t("invite.INVALID_LINK")}</AlertDescription>
        </Alert>
        <Link to="/login" className="text-signal">
          {t("goLogin")}
        </Link>
      </div>
    );
  }

  const boundEmail = loaderData.email;
  const expires = new Intl.DateTimeFormat(i18n.language, { dateStyle: "long" }).format(
    new Date(loaderData.expiresAt),
  );
  return (
    <form
      onSubmit={form.handleSubmit((v) => submit(v, { method: "post", replace: true }))}
      noValidate
      className="flex flex-col gap-6"
    >
      <div className="flex flex-col gap-2">
        <h2 className="font-serif text-4xl font-medium">{t("invite.title")}</h2>
        <p className="text-muted-foreground">{t("invite.subtitle")}</p>
        <p className="text-sm text-subtle-foreground">
          {t("invite.role", { role: t(`common:roles.${loaderData.role}`), date: expires })}
        </p>
      </div>
      {result?.formError && (
        <Alert variant="destructive">
          <AlertDescription>
            {t(`invite.${result.formError}`, {
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
        readOnly={boundEmail !== null}
        hint={boundEmail !== null ? t("invite.emailHint") : undefined}
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
        {t("invite.submit")}
      </Button>
    </form>
  );
}
