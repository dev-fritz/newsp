import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { FormProvider, useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import type { ChatModel, Preferences, SourceSummary } from "@/features/profile/api/profile-api";
import { useSavePreferences } from "@/features/profile/api/profile-queries";
import { AppearanceSection } from "@/features/profile/components/appearance-section";
import { IdeasSection } from "@/features/profile/components/ideas-section";
import { InterestsSection } from "@/features/profile/components/interests-section";
import { ModelsSection } from "@/features/profile/components/models-section";
import { ReportSection } from "@/features/profile/components/report-section";
import { SecuritySection } from "@/features/profile/components/security-section";
import { SourcesSection } from "@/features/profile/components/sources-section";
import { UnsavedBar } from "@/features/profile/components/unsaved-bar";
import { toFormValues, toRequest } from "@/features/profile/schemas/preferences-mapper";
import {
  preferencesSchema,
  type PreferencesValues,
} from "@/features/profile/schemas/profile.schema";
import { applyTheme } from "@/lib/theme";

const FORM_ID = "preferences-form";

type PreferencesFormProps = {
  preferences: Preferences;
  models: ChatModel[];
  sources: SourceSummary[];
};

export function PreferencesForm({ preferences, models, sources }: PreferencesFormProps) {
  const { t } = useTranslation(["profile", "common"]);
  const save = useSavePreferences();
  // Last saved server state: base of the next request and of the next-report time.
  const [saved, setSaved] = useState(preferences);
  const form = useForm<PreferencesValues>({
    resolver: zodResolver(preferencesSchema),
    defaultValues: toFormValues(preferences, sources),
  });
  const { isDirty, dirtyFields } = form.formState;
  const changes = isDirty ? Object.keys(dirtyFields).length : 0;

  const onSubmit = form.handleSubmit((values) => {
    const localeChanged = values.ui_locale !== saved.ui_locale;
    save.mutate(toRequest(values, saved, sources), {
      onSuccess: (result) => {
        setSaved(result);
        form.reset(toFormValues(result, sources));
        applyTheme(values.theme);
        toast.success(t("profile:toast.saved"));
        // SSR renders the interface language, so reload to pick up the new one.
        if (localeChanged) window.location.reload();
      },
      onError: (error) => {
        for (const [field, tag] of Object.entries(error.fields ?? {})) {
          const name = field === "excluded_source_ids" ? "included_source_ids" : field;
          if (name in values) form.setError(name as keyof PreferencesValues, { message: tag });
        }
        toast.error(t(`common:errors.${error.code}`, { defaultValue: t("profile:toast.failed") }));
      },
    });
  });

  return (
    <FormProvider {...form}>
      <form id={FORM_ID} onSubmit={onSubmit} noValidate className="flex flex-col">
        <InterestsSection topicsRanked={saved.topics_ranked ?? true} />
        <ReportSection nextReportAt={saved.next_report_at} savedTimezone={saved.timezone} />
        <IdeasSection />
        <ModelsSection models={models} />
        <SourcesSection sources={sources} />
        <AppearanceSection />
      </form>
      <SecuritySection />
      <UnsavedBar
        formId={FORM_ID}
        count={changes}
        saving={save.isPending}
        onDiscard={() => {
          form.reset();
          applyTheme(form.getValues("theme"));
        }}
      />
    </FormProvider>
  );
}
