import { zodResolver } from "@hookform/resolvers/zod";
import { useMemo } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { FormField } from "@/components/common/form-field";
import { Button } from "@/components/ui/button";

import { useUpdateProvider } from "../api/ai-queries";
import type { UpdateProviderBody } from "../api/ai-api";
import { useErrorToast } from "../hooks/use-error-toast";
import { applyFieldErrors, byFieldMap } from "../lib/field-errors";
import {
  buildProviderSchema,
  PROVIDER_FIELD_MAP,
  type ProviderFormValues,
} from "../schemas/provider.schema";
import type { Provider } from "../types";

type ProviderEditFormProps = {
  provider: Provider;
  /** Only the API key (the "update key" action). */
  keyOnly?: boolean;
  onDone: () => void;
  onCancel: () => void;
};

const mapFields = byFieldMap(PROVIDER_FIELD_MAP);

/** Edits name, base URL and (write-only) API key of a provider. A blank key keeps the current one. */
export function ProviderEditForm({ provider, keyOnly, onDone, onCancel }: ProviderEditFormProps) {
  const { t } = useTranslation("admin");
  const toastError = useErrorToast();
  const update = useUpdateProvider();
  const schema = useMemo(() => buildProviderSchema(false), []);

  const form = useForm<ProviderFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      kind: provider.kind,
      name: provider.name,
      baseUrl: provider.base_url ?? "",
      apiKey: "",
    },
  });
  const { errors } = form.formState;

  async function submit(v: ProviderFormValues) {
    const body: UpdateProviderBody = keyOnly
      ? { api_key: v.apiKey }
      : {
          name: v.name,
          base_url: v.baseUrl,
          // A blank key means "keep the current one": leave it out.
          ...(v.apiKey ? { api_key: v.apiKey } : {}),
        };
    if (keyOnly && !v.apiKey) {
      form.setError("apiKey", { message: "required" });
      return;
    }
    try {
      await update.mutateAsync({ id: provider.id, body });
      toast.success(
        t(keyOnly ? "ai.toast.keyUpdated" : "ai.toast.updated", { name: provider.name }),
      );
      onDone();
    } catch (error) {
      if (!applyFieldErrors(error, mapFields, (f, e) => form.setError(f, e))) toastError(error);
    }
  }

  return (
    <form onSubmit={form.handleSubmit(submit)} noValidate className="flex flex-col gap-5">
      {!keyOnly && (
        <>
          <FormField
            label={t("ai.form.name")}
            autoComplete="off"
            error={errors.name?.message}
            {...form.register("name")}
          />
          <FormField
            label={t("ai.form.baseUrl")}
            type="url"
            inputMode="url"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            className="h-10 font-mono text-[13px]"
            hint={
              provider.kind === "openai_compatible"
                ? t("ai.form.baseUrlRequired")
                : t("ai.form.baseUrlOptional")
            }
            error={errors.baseUrl?.message}
            {...form.register("baseUrl")}
          />
        </>
      )}
      <FormField
        label={t("ai.form.newKey")}
        type="password"
        autoComplete="off"
        className="h-10 font-mono text-[13px]"
        placeholder={t("ai.form.apiKeyPlaceholder")}
        hint={
          provider.key_hint
            ? t("ai.form.newKeyHint", { hint: provider.key_hint })
            : t("ai.form.apiKeyHint")
        }
        error={errors.apiKey?.message}
        {...form.register("apiKey")}
      />
      <div className="flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onCancel} className="pointer-coarse:h-11">
          {t("common:actions.cancel")}
        </Button>
        <Button type="submit" disabled={update.isPending} className="pointer-coarse:h-11">
          {t("common:actions.save")}
        </Button>
      </div>
    </form>
  );
}
