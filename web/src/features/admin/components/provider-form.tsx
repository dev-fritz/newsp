import { zodResolver } from "@hookform/resolvers/zod";
import { useMemo } from "react";
import { Controller, useForm, useWatch } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { FormField } from "@/components/common/form-field";
import { Button } from "@/components/ui/button";

import { useCreateProvider, useTestProvider } from "../api/ai-queries";
import { useErrorToast } from "../hooks/use-error-toast";
import { applyFieldErrors, byFieldMap } from "../lib/field-errors";
import { DEFAULT_PROVIDER_NAME, OLLAMA_BASE_URL } from "../lib/provider-kinds";
import {
  buildProviderSchema,
  PROVIDER_FIELD_MAP,
  type ProviderFormValues,
} from "../schemas/provider.schema";
import type { ConnectionTest, Provider, ProviderKind } from "../types";
import { ProviderKindPicker } from "./provider-kind-picker";

type ProviderFormProps = {
  /** Called once the provider exists and its connection was tested (test is null if the test call itself failed). */
  onConnected: (provider: Provider, test: ConnectionTest | null) => void;
  onCancel?: () => void;
  recommendLocal?: boolean;
  submitLabel?: string;
};

const mapFields = byFieldMap(PROVIDER_FIELD_MAP);

/** Registers a provider, then tests it right away. Shared by the admin dialog and the setup wizard. */
export function ProviderForm({
  onConnected,
  onCancel,
  recommendLocal,
  submitLabel,
}: ProviderFormProps) {
  const { t } = useTranslation("admin");
  const toastError = useErrorToast();
  const create = useCreateProvider();
  const test = useTestProvider();
  const pending = create.isPending || test.isPending;
  const schema = useMemo(() => buildProviderSchema(true), []);

  const form = useForm<ProviderFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      kind: "openai_compatible",
      name: DEFAULT_PROVIDER_NAME.openai_compatible,
      baseUrl: OLLAMA_BASE_URL,
      apiKey: "",
    },
  });
  const { errors, dirtyFields } = form.formState;
  const kind = useWatch({ control: form.control, name: "kind" });

  function pickKind(next: ProviderKind) {
    form.setValue("kind", next);
    form.setValue("baseUrl", next === "openai_compatible" ? OLLAMA_BASE_URL : "");
    if (!dirtyFields.name) form.setValue("name", DEFAULT_PROVIDER_NAME[next]);
    form.clearErrors();
  }

  async function submit(v: ProviderFormValues) {
    let provider: Provider;
    try {
      provider = await create.mutateAsync({
        name: v.name,
        kind: v.kind,
        base_url: v.baseUrl || undefined,
        api_key: v.apiKey || undefined,
      });
    } catch (error) {
      if (!applyFieldErrors(error, mapFields, (f, e) => form.setError(f, e))) toastError(error);
      return;
    }
    toast.success(t("ai.toast.created", { name: provider.name }));
    try {
      onConnected(provider, await test.mutateAsync(provider.id));
    } catch {
      onConnected(provider, null);
    }
  }

  return (
    <form onSubmit={form.handleSubmit(submit)} noValidate className="flex flex-col gap-5">
      <Controller
        control={form.control}
        name="kind"
        render={({ field }) => (
          <ProviderKindPicker
            value={field.value}
            onChange={pickKind}
            recommendLocal={recommendLocal}
          />
        )}
      />
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
        placeholder={kind === "openai_compatible" ? OLLAMA_BASE_URL : t("ai.form.baseUrlDefault")}
        hint={
          kind === "openai_compatible" ? t("ai.form.baseUrlRequired") : t("ai.form.baseUrlOptional")
        }
        error={errors.baseUrl?.message}
        {...form.register("baseUrl")}
      />
      <FormField
        label={kind === "openai_compatible" ? t("ai.form.apiKeyOptional") : t("ai.form.apiKey")}
        type="password"
        autoComplete="off"
        className="h-10 font-mono text-[13px]"
        placeholder={t("ai.form.apiKeyPlaceholder")}
        hint={t("ai.form.apiKeyHint")}
        error={errors.apiKey?.message}
        {...form.register("apiKey")}
      />
      <div className="flex flex-wrap justify-end gap-2">
        {onCancel && (
          <Button type="button" variant="ghost" onClick={onCancel} className="pointer-coarse:h-11">
            {t("common:actions.cancel")}
          </Button>
        )}
        <Button type="submit" disabled={pending} className="pointer-coarse:h-11">
          {pending ? t("ai.form.connecting") : (submitLabel ?? t("ai.form.connect"))}
        </Button>
      </div>
    </form>
  );
}
