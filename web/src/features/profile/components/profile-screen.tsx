import { useTranslation } from "react-i18next";

import { ErrorState, ListSkeleton } from "@/components/common/states";
import { PageHeader } from "@/components/common/page-header";
import {
  useChatModels,
  usePreferences,
  useSourceOptions,
} from "@/features/profile/api/profile-queries";
import { PreferencesForm } from "@/features/profile/components/preferences-form";

export function ProfileScreen() {
  const { t } = useTranslation("profile");
  const prefs = usePreferences();
  const models = useChatModels();
  const sources = useSourceOptions();

  const error = prefs.error ?? models.error ?? sources.error;
  let body: React.ReactNode;
  if (error) {
    body = (
      <ErrorState
        error={error}
        onRetry={() => {
          void prefs.refetch();
          void models.refetch();
          void sources.refetch();
        }}
      />
    );
  } else if (!prefs.data || !models.data || !sources.data) {
    body = <ListSkeleton rows={5} />;
  } else {
    body = <PreferencesForm preferences={prefs.data} models={models.data} sources={sources.data} />;
  }

  return (
    <div className="mx-auto flex w-full max-w-240 flex-col gap-7">
      <PageHeader kicker={t("kicker")} title={t("title")} description={t("description")} />
      {body}
    </div>
  );
}
