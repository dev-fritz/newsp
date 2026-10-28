import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Alert, AlertDescription } from "@/components/ui/alert";
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

  // The admin turned BYOK off: nothing to connect. Keep the section only while stored keys
  // remain, so the user can still delete them (the API allows DELETE with BYOK off).
  const data = query.data;
  const byokOff = data?.byok_enabled === false;
  if (data && byokOff && data.providers.length === 0) return null;

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
          {byokOff && (
            <Alert>
              <AlertDescription>{t("ownProviders.disabledNotice")}</AlertDescription>
            </Alert>
          )}
          {OWN_PROVIDER_KINDS.map((kind) => {
            const provider = query.data.providers.find((p) => p.kind === kind);
            return provider ? (
              <OwnProviderCard key={kind} provider={provider} readOnly={byokOff} />
            ) : byokOff ? null : (
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
