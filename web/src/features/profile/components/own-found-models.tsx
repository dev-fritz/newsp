import { Check } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { OwnProvider } from "@/features/profile/api/profile-api";
import { useCreateOwnModel } from "@/features/profile/api/profile-queries";
import { useOwnErrorToast } from "@/features/profile/hooks/use-own-error-toast";

const FILTER_FROM = 8;

type OwnFoundModelsProps = { provider: OwnProvider; names: string[] };

/** Embedding models are not offered: user providers are chat only. */
const EMBEDDING = /embed/i;

/** Models the provider reported, each addable as a chat model in one click (no embeddings). */
export function OwnFoundModels({ provider, names: found }: OwnFoundModelsProps) {
  const { t } = useTranslation("profile");
  const toastError = useOwnErrorToast();
  const create = useCreateOwnModel();
  const [filter, setFilter] = useState("");

  const names = found.filter((n) => !EMBEDDING.test(n));
  const registered = new Set(provider.models.map((m) => m.name));
  const needle = filter.trim().toLowerCase();
  const shown = names.filter((n) => n.toLowerCase().includes(needle));
  const adding = create.isPending ? create.variables.body.name : null;

  function add(name: string) {
    create.mutate(
      { providerId: provider.id, body: { name } },
      {
        onSuccess: () => toast.success(t("ownProviders.found.added", { name })),
        onError: toastError,
      },
    );
  }

  return (
    <div className="flex flex-col gap-2">
      {names.length > FILTER_FROM && (
        <Input
          type="search"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder={t("ownProviders.found.filter")}
          aria-label={t("ownProviders.found.filter")}
          className="h-9"
        />
      )}
      <ul className="flex max-h-72 flex-col divide-y divide-border overflow-y-auto rounded-lg border border-border">
        {shown.map((name) => (
          <li key={name} className="flex flex-wrap items-center justify-between gap-2 px-3 py-2">
            <span className="min-w-0 truncate font-mono text-[13px]" title={name}>
              {name}
            </span>
            {registered.has(name) ? (
              <Badge variant="outline" className="gap-1">
                <Check aria-hidden />
                {t("ownProviders.found.registered")}
              </Badge>
            ) : (
              <Button
                size="sm"
                disabled={adding !== null}
                onClick={() => add(name)}
                className="pointer-coarse:h-11"
                aria-label={t("ownProviders.found.addNamed", { name })}
              >
                {t("ownProviders.found.add")}
              </Button>
            )}
          </li>
        ))}
        {shown.length === 0 && (
          <li className="px-3 py-3 text-sm text-muted-foreground">
            {t("ownProviders.found.noMatch")}
          </li>
        )}
      </ul>
      <p className="text-xs text-subtle-foreground">{t("ownProviders.found.hint")}</p>
    </div>
  );
}
