import { useState } from "react";
import { useTranslation } from "react-i18next";

import { ErrorState, ListSkeleton } from "@/components/common/states";
import type { OwnProviderKind } from "@/features/profile/api/profile-api";
import { useOwnProviders } from "@/features/profile/api/profile-queries";
import { ConnectProviderCard } from "@/features/profile/components/connect-provider-card";
import { ConnectProviderDialog } from "@/features/profile/components/connect-provider-dialog";
import { OwnProviderCard } from "@/features/profile/components/own-provider-card";
import { SettingsSection } from "@/features/profile/components/settings-section";
import { OWN_PROVIDER_KINDS } from "@/features/profile/schemas/own-provider.schema";

/** "My AI providers" (BYOK). Has its own mutations, so it lives outside the preferences form. */
export function OwnProvidersSection() {
  const { t } = useTranslation("profile");
  const query = useOwnProviders();
  const [connecting, setConnecting] = useState<OwnProviderKind | null>(null);

  // The admin turned BYOK off: the whole section is hidden.
  if (query.data && !query.data.byok_enabled) return null;

  return (
    <SettingsSection
      id="s-own-providers"
      title={t("ownProviders.title")}
      hint={t("ownProviders.hint")}
    >
      {query.isPending ? (
        <ListSkeleton rows={3} />
      ) : query.isError ? (
        <ErrorState error={query.error} onRetry={() => void query.refetch()} />
      ) : (
        <>
          {OWN_PROVIDER_KINDS.map((kind) => {
            const provider = query.data.providers.find((p) => p.kind === kind);
            return provider ? (
              <OwnProviderCard key={kind} provider={provider} />
            ) : (
              <ConnectProviderCard key={kind} kind={kind} onConnect={() => setConnecting(kind)} />
            );
          })}
          {connecting && (
            <ConnectProviderDialog
              kind={connecting}
              open
              onOpenChange={(open) => !open && setConnecting(null)}
            />
          )}
        </>
      )}
    </SettingsSection>
  );
}
