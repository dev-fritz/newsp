import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";

import { FormField } from "@/components/common/form-field";
import { Button } from "@/components/ui/button";
import { useOwnErrorToast } from "@/features/profile/hooks/use-own-error-toast";
import { ownKeySchema, type OwnKeyValues } from "@/features/profile/schemas/own-provider.schema";
import { isApiError } from "@/lib/api-error";

type OwnKeyFormProps = {
  hint: string;
  submitLabel: string;
  pendingLabel: string;
  /** Throws an ApiError on failure; a `api_key` field error lands on the input, others on a toast. */
  onSubmit: (apiKey: string) => Promise<void>;
  onCancel: () => void;
};

/** Write-only API key form shared by "connect" and "replace key". */
export function OwnKeyForm({
  hint,
  submitLabel,
  pendingLabel,
  onSubmit,
  onCancel,
}: OwnKeyFormProps) {
  const { t } = useTranslation(["profile", "common"]);
  const toastError = useOwnErrorToast();
  const form = useForm<OwnKeyValues>({
    resolver: zodResolver(ownKeySchema),
    defaultValues: { api_key: "" },
  });
  const { errors, isSubmitting } = form.formState;

  const submit = form.handleSubmit(async (values) => {
    try {
      await onSubmit(values.api_key);
    } catch (error) {
      const tag = isApiError(error) ? error.fields?.api_key : undefined;
      if (tag) form.setError("api_key", { message: tag });
      else toastError(error);
    }
  });

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-5">
      <FormField
        label={t("profile:ownProviders.form.apiKey")}
        type="password"
        autoComplete="off"
        autoCapitalize="none"
        autoCorrect="off"
        spellCheck={false}
        className="h-10 font-mono text-[13px]"
        hint={hint}
        error={errors.api_key?.message}
        {...form.register("api_key")}
      />
      <div className="flex flex-wrap justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onCancel} className="pointer-coarse:h-11">
          {t("common:actions.cancel")}
        </Button>
        <Button type="submit" disabled={isSubmitting} className="pointer-coarse:h-11">
          {isSubmitting ? pendingLabel : submitLabel}
        </Button>
      </div>
    </form>
  );
}
