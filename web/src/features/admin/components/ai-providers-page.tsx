import { useTranslation } from "react-i18next";

import { EmptyState, ErrorState, ListSkeleton } from "@/components/common/states";
import { PageHeader } from "@/components/common/page-header";
import { useCan } from "@/features/auth";

import { useProviders, useSettings } from "../api/ai-queries";
import { errorOf, FORBIDDEN } from "../lib/error-of";
import { AddProviderButton } from "./add-provider-button";
import { InstanceSettings } from "./instance-settings";
import { ModelsTable } from "./models-table";
import { ProvidersGrid } from "./providers-grid";

/** /admin/ai: providers, their models and the instance-wide AI settings. */
export function AiProvidersPage() {
  const { t } = useTranslation("admin");
  const canProviders = useCan("providers:manage");
  const canSettings = useCan("settings:manage");
  const providers = useProviders();
  // Shared with InstanceSettings (same query key): the default badges of the models table.
  const settings = useSettings(canSettings);

  const header = (
    <PageHeader kicker={t("kicker")} title={t("ai.title")} description={t("ai.description")} />
  );

  if (!canProviders) {
    return (
      <div className="flex flex-col gap-6">
        {header}
        <ErrorState error={FORBIDDEN} />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-7">
      {header}
      {providers.isPending && <ListSkeleton rows={3} />}
      {providers.isError && (
        <ErrorState error={errorOf(providers.error)} onRetry={() => void providers.refetch()} />
      )}
      {providers.data && providers.data.length === 0 && (
        <EmptyState
          title={t("ai.empty.title")}
          body={t("ai.empty.body")}
          action={<AddProviderButton />}
        />
      )}
      {providers.data && providers.data.length > 0 && (
        <>
          <ProvidersGrid providers={providers.data} />
          <ModelsTable providers={providers.data} settings={settings.data} />
          {canSettings && <InstanceSettings providers={providers.data} />}
        </>
      )}
    </div>
  );
}
