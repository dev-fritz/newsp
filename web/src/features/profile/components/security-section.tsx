import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { Form, useFetcher } from "react-router";
import { toast } from "sonner";

import { FormField } from "@/components/common/form-field";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { SettingsSection } from "@/features/profile/components/settings-section";
import {
  passwordFormSchema,
  type PasswordActionResult,
  type PasswordFormValues,
} from "@/features/profile/schemas/profile.schema";

/** Server-side change of password (the route action keeps tokens off the browser). */
export function SecuritySection() {
  const { t } = useTranslation(["profile", "common"]);
  const fetcher = useFetcher<PasswordActionResult>();
  const form = useForm<PasswordFormValues>({
    resolver: zodResolver(passwordFormSchema),
    defaultValues: { current_password: "", new_password: "", confirm: "" },
  });
  const { errors } = form.formState;
  const result = fetcher.data;
  const submitting = fetcher.state !== "idle";

  useEffect(() => {
    if (!result) return;
    if (result.done) {
      toast.success(t("profile:security.done"));
      form.reset();
      return;
    }
    for (const [field, tag] of Object.entries(result.fields ?? {})) {
      if (field === "current_password" || field === "new_password") {
        form.setError(field, { message: tag });
      }
    }
  }, [result, form, t]);

  const onSubmit = form.handleSubmit((values) =>
    fetcher.submit(
      { current_password: values.current_password, new_password: values.new_password },
      { method: "post", action: "/profile" },
    ),
  );

  const formError = result?.formError;
  return (
    <SettingsSection
      id="s-security"
      title={t("profile:security.title")}
      hint={t("profile:security.hint")}
    >
      <form onSubmit={onSubmit} noValidate className="flex flex-col gap-3.5">
        {formError && (
          <Alert variant="destructive">
            <AlertDescription>
              {t(`profile:security.${formError}`, {
                defaultValue: t(`common:errors.${formError}`, {
                  defaultValue: t("common:errors.UNKNOWN_ERROR"),
                }),
              })}
            </AlertDescription>
          </Alert>
        )}
        <FormField
          label={t("profile:security.current")}
          type="password"
          autoComplete="current-password"
          error={errors.current_password?.message}
          {...form.register("current_password")}
        />
        <div className="grid grid-cols-[repeat(auto-fit,minmax(min(200px,100%),1fr))] gap-3.5">
          <FormField
            label={t("profile:security.new")}
            hint={t("profile:security.newHint")}
            type="password"
            autoComplete="new-password"
            error={errors.new_password?.message}
            {...form.register("new_password")}
          />
          <FormField
            label={t("profile:security.confirm")}
            type="password"
            autoComplete="new-password"
            error={errors.confirm?.message}
            {...form.register("confirm")}
          />
        </div>
        <div className="flex flex-wrap gap-2">
          <Button type="submit" variant="secondary" disabled={submitting}>
            {submitting ? t("profile:security.submitting") : t("profile:security.submit")}
          </Button>
        </div>
      </form>
      <Form method="post" action="/logout">
        <Button type="submit" variant="outline">
          {t("profile:security.signOut")}
        </Button>
      </Form>
    </SettingsSection>
  );
}
