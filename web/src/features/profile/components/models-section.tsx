import { Controller, useFormContext } from "react-hook-form";
import { useTranslation } from "react-i18next";

import type { ChatModel } from "@/features/profile/api/profile-api";
import { ModelSelect } from "@/features/profile/components/model-select";
import { SettingsSection } from "@/features/profile/components/settings-section";
import type { PreferencesValues } from "@/features/profile/schemas/profile.schema";

export function ModelsSection({ models }: { models: ChatModel[] }) {
  const { t } = useTranslation("profile");
  const { control } = useFormContext<PreferencesValues>();
  return (
    <SettingsSection id="s-models" title={t("models.title")} hint={t("models.hint")}>
      <Controller
        control={control}
        name="report_model_id"
        render={({ field }) => (
          <ModelSelect
            id="pref-model-report"
            label={t("models.report")}
            models={models}
            value={field.value}
            onChange={field.onChange}
          />
        )}
      />
      <Controller
        control={control}
        name="idea_model_id"
        render={({ field }) => (
          <ModelSelect
            id="pref-model-idea"
            label={t("models.idea")}
            models={models}
            value={field.value}
            onChange={field.onChange}
          />
        )}
      />
    </SettingsSection>
  );
}
