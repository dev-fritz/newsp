import { Controller, useFormContext } from "react-hook-form";
import { useTranslation } from "react-i18next";

import { SettingsSection } from "@/features/profile/components/settings-section";
import { TopicsInput } from "@/features/profile/components/topics-input";
import type { PreferencesValues } from "@/features/profile/schemas/profile.schema";
import { Alert, AlertDescription } from "@/components/ui/alert";

export function InterestsSection({ topicsRanked }: { topicsRanked: boolean }) {
  const { t } = useTranslation("profile");
  const { control } = useFormContext<PreferencesValues>();
  return (
    <SettingsSection id="s-interests" title={t("interests.title")} hint={t("interests.hint")}>
      <Controller
        control={control}
        name="topics"
        render={({ field }) => <TopicsInput value={field.value} onChange={field.onChange} />}
      />
      {!topicsRanked && (
        <Alert>
          <AlertDescription>{t("interests.notRanked")}</AlertDescription>
        </Alert>
      )}
    </SettingsSection>
  );
}
