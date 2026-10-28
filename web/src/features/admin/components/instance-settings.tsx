import { zodResolver } from "@hookform/resolvers/zod";
import { AlertCircle } from "lucide-react";
import { useMemo, useState } from "react";
import { Controller, useForm, useWatch } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { isApiError } from "@/lib/api-error";

import { ErrorState } from "@/components/common/states";
import { FormField } from "@/components/common/form-field";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";

import type { UpdateSettingsBody } from "../api/ai-api";
import { useSettings, useUpdateSettings } from "../api/ai-queries";
import { useErrorToast } from "../hooks/use-error-toast";
import { useFormat } from "../hooks/use-format";
import { applyFieldErrors, byFieldMap } from "../lib/field-errors";
import { errorOf } from "../lib/error-of";
import { modelOptions } from "../lib/model-options";
import {
  MAX_INACTIVITY,
  MAX_RETENTION,
  MAX_THRESHOLD,
  MIN_INACTIVITY,
  MIN_RETENTION,
  MIN_THRESHOLD,
  SETTINGS_FIELD_MAP,
  settingsFormSchema,
  type SettingsFormValues,
} from "../schemas/settings.schema";
import type { InstanceSettings as Settings, Provider } from "../types";
import { ReindexNotice } from "./reindex-notice";
import { SelectField } from "./select-field";
import { SettingsSection } from "./settings-section";

const mapFields = byFieldMap(SETTINGS_FIELD_MAP);

function toValues(s: Settings): SettingsFormValues {
  return {
    embeddingModelId: s.embedding_model_id ?? "",
    reportModelId: s.default_report_model_id ?? "",
    ideaModelId: s.default_idea_model_id ?? "",
    retentionDays: String(s.retention_days ?? 90),
    threshold: (s.idea_similarity_threshold ?? 0.8).toFixed(2),
    signupEnabled: s.signup_enabled ?? false,
    byokEnabled: s.byok_enabled ?? true,
    inactivityDays: String(s.inactivity_pause_days ?? 0),
  };
}

function SettingsForm({ settings, providers }: { settings: Settings; providers: Provider[] }) {
  const { t } = useTranslation("admin");
  const format = useFormat();
  const toastError = useErrorToast();
  const update = useUpdateSettings();
  const [reindexJob, setReindexJob] = useState<string | null>(null);
  const [mailerError, setMailerError] = useState(false);

  const values = useMemo(() => toValues(settings), [settings]);
  const form = useForm<SettingsFormValues>({
    resolver: zodResolver(settingsFormSchema),
    values,
  });
  const { errors, isDirty } = form.formState;
  const threshold = Number(useWatch({ control: form.control, name: "threshold" }));

  const chat = modelOptions(providers, "chat");
  const embedding = modelOptions(providers, "embedding");

  async function submit(v: SettingsFormValues) {
    const body: UpdateSettingsBody = {};
    if (v.embeddingModelId && v.embeddingModelId !== settings.embedding_model_id)
      body.embedding_model_id = v.embeddingModelId;
    if (v.reportModelId && v.reportModelId !== settings.default_report_model_id)
      body.default_report_model_id = v.reportModelId;
    if (v.ideaModelId && v.ideaModelId !== settings.default_idea_model_id)
      body.default_idea_model_id = v.ideaModelId;
    if (Number(v.retentionDays) !== settings.retention_days)
      body.retention_days = Number(v.retentionDays);
    if (Math.abs(Number(v.threshold) - (settings.idea_similarity_threshold ?? 0)) > 0.0001)
      body.idea_similarity_threshold = Number(v.threshold);
    if (v.signupEnabled !== (settings.signup_enabled ?? false))
      body.signup_enabled = v.signupEnabled;
    if (v.byokEnabled !== (settings.byok_enabled ?? true)) body.byok_enabled = v.byokEnabled;
    if (Number(v.inactivityDays) !== (settings.inactivity_pause_days ?? 0))
      body.inactivity_pause_days = Number(v.inactivityDays);
    setMailerError(false);
    try {
      const saved = await update.mutateAsync(body);
      toast.success(t("ai.settings.saved"));
      if (saved.reindex_job_id) setReindexJob(saved.reindex_job_id);
    } catch (error) {
      if (isApiError(error) && error.code === "MAILER_NOT_CONFIGURED") {
        setMailerError(true);
        return;
      }
      if (!applyFieldErrors(error, mapFields, (f, e) => form.setError(f, e))) toastError(error);
    }
  }

  return (
    <form onSubmit={form.handleSubmit(submit)} noValidate className="flex flex-col">
      <SettingsSection
        title={t("ai.settings.embeddings.title")}
        description={t("ai.settings.embeddings.body")}
      >
        <Controller
          control={form.control}
          name="embeddingModelId"
          render={({ field }) => (
            <SelectField
              label={t("ai.settings.embeddings.label")}
              value={field.value}
              onValueChange={field.onChange}
              options={embedding}
              placeholder={t("ai.settings.pick")}
              hint={embedding.length === 0 ? t("ai.settings.noEmbedding") : undefined}
              error={errors.embeddingModelId?.message}
            />
          )}
        />
        <div className="flex gap-2.5 rounded-xl bg-warning-soft p-3 text-[13px] leading-snug text-warning">
          <AlertCircle className="mt-px size-4 shrink-0" aria-hidden />
          <span>{t("ai.settings.embeddings.warning")}</span>
        </div>
        {reindexJob && <ReindexNotice jobId={reindexJob} />}
      </SettingsSection>

      <SettingsSection
        title={t("ai.settings.defaults.title")}
        description={t("ai.settings.defaults.body")}
      >
        <div className="grid grid-cols-[repeat(auto-fit,minmax(min(200px,100%),1fr))] gap-3.5">
          <Controller
            control={form.control}
            name="reportModelId"
            render={({ field }) => (
              <SelectField
                label={t("ai.settings.defaults.reports")}
                value={field.value}
                onValueChange={field.onChange}
                options={chat}
                placeholder={t("ai.settings.pick")}
                error={errors.reportModelId?.message}
              />
            )}
          />
          <Controller
            control={form.control}
            name="ideaModelId"
            render={({ field }) => (
              <SelectField
                label={t("ai.settings.defaults.ideas")}
                value={field.value}
                onValueChange={field.onChange}
                options={chat}
                placeholder={t("ai.settings.pick")}
                error={errors.ideaModelId?.message}
              />
            )}
          />
        </div>
      </SettingsSection>

      <SettingsSection title={t("ai.settings.byok.title")} description={t("ai.settings.byok.body")}>
        <Controller
          control={form.control}
          name="byokEnabled"
          render={({ field }) => (
            <div className="flex items-start justify-between gap-4">
              <Label htmlFor="byok-enabled" className="flex flex-col items-start gap-1">
                {t("ai.settings.byok.label")}
                <span className="text-xs font-normal text-subtle-foreground">
                  {t("ai.settings.byok.hint")}
                </span>
              </Label>
              <Switch
                id="byok-enabled"
                checked={field.value}
                onCheckedChange={field.onChange}
                className="mt-0.5 pointer-coarse:h-6 pointer-coarse:w-11"
              />
            </div>
          )}
        />
      </SettingsSection>

      <SettingsSection
        title={t("ai.settings.similarity.title")}
        description={t("ai.settings.similarity.body")}
      >
        <div className="flex flex-col gap-2">
          <Label htmlFor="threshold" className="justify-between">
            {t("ai.settings.similarity.label")}
            <span className="font-mono">
              {Number.isFinite(threshold) ? format.number(threshold) : "—"}
            </span>
          </Label>
          <input
            id="threshold"
            type="range"
            min={MIN_THRESHOLD}
            max={MAX_THRESHOLD}
            step={0.01}
            className="h-6 w-full accent-signal"
            aria-invalid={errors.threshold ? true : undefined}
            {...form.register("threshold")}
          />
          <span className="flex justify-between text-xs text-subtle-foreground">
            <span>{t("ai.settings.similarity.low")}</span>
            <span>{t("ai.settings.similarity.high")}</span>
          </span>
        </div>
      </SettingsSection>

      <SettingsSection
        title={t("ai.settings.retention.title")}
        description={t("ai.settings.retention.body")}
      >
        <div className="flex flex-col gap-1.5">
          <FormField
            label={t("ai.settings.retention.label")}
            type="number"
            inputMode="numeric"
            min={MIN_RETENTION}
            max={MAX_RETENTION}
            className="h-10 w-32"
            hint={t("ai.settings.retention.hint", { min: MIN_RETENTION, max: MAX_RETENTION })}
            error={errors.retentionDays?.message}
            {...form.register("retentionDays")}
          />
        </div>
      </SettingsSection>

      <SettingsSection
        title={t("ai.settings.accounts.title")}
        description={t("ai.settings.accounts.body")}
      >
        <Controller
          control={form.control}
          name="signupEnabled"
          render={({ field }) => {
            const locked = !settings.mail_configured && !field.value;
            return (
              <div className="flex flex-col gap-1.5">
                <div className="flex items-start justify-between gap-4">
                  <Label htmlFor="signup-enabled" className="flex flex-col items-start gap-1">
                    {t("ai.settings.accounts.signup")}
                    <span className="text-xs font-normal text-subtle-foreground">
                      {t("ai.settings.accounts.signupBody")}
                    </span>
                  </Label>
                  <Switch
                    id="signup-enabled"
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={locked}
                    aria-describedby={locked ? "signup-hint" : undefined}
                    className="mt-0.5 pointer-coarse:h-6 pointer-coarse:w-11"
                  />
                </div>
                {locked && (
                  <p id="signup-hint" className="text-xs text-warning">
                    {t("ai.settings.accounts.noMail")}
                  </p>
                )}
                {mailerError && (
                  <p className="text-xs text-negative" role="alert">
                    {t("errors.MAILER_NOT_CONFIGURED")}
                  </p>
                )}
              </div>
            );
          }}
        />
        <FormField
          label={t("ai.settings.accounts.inactivity")}
          type="number"
          inputMode="numeric"
          min={MIN_INACTIVITY}
          max={MAX_INACTIVITY}
          className="h-10 w-32"
          hint={t("ai.settings.accounts.inactivityHint")}
          error={errors.inactivityDays?.message}
          {...form.register("inactivityDays")}
        />
      </SettingsSection>

      <div className="flex justify-end border-t border-border pt-5">
        <Button
          type="submit"
          disabled={update.isPending || !isDirty}
          className="pointer-coarse:h-11"
        >
          {t("ai.settings.save")}
        </Button>
      </div>
    </form>
  );
}

/** Instance-wide AI settings: embedding model, default models, similarity and retention. */
export function InstanceSettings({ providers }: { providers: Provider[] }) {
  const { t } = useTranslation("admin");
  const settings = useSettings();
  return (
    <section aria-labelledby="instance-title" className="flex flex-col">
      <h2 id="instance-title" className="pb-2 font-serif text-[26px] font-medium">
        {t("ai.settings.title")}
      </h2>
      {settings.isPending && <Skeleton className="h-64 w-full rounded-2xl" aria-hidden />}
      {settings.isError && (
        <ErrorState error={errorOf(settings.error)} onRetry={() => void settings.refetch()} />
      )}
      {settings.data && <SettingsForm settings={settings.data} providers={providers} />}
    </section>
  );
}
