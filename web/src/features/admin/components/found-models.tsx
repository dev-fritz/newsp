import { Check } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

import { useCreateModel } from "../api/ai-queries";
import { useErrorToast } from "../hooks/use-error-toast";
import { supportsEmbeddings } from "../lib/provider-status";
import type { Capability, Provider } from "../types";

const FILTER_FROM = 8;

type FoundModelsProps = { provider: Provider; names: string[] };

/** Models the provider reported, each addable as chat or embedding in one click. */
export function FoundModels({ provider, names }: FoundModelsProps) {
  const { t } = useTranslation("admin");
  const toastError = useErrorToast();
  const create = useCreateModel();
  const [filter, setFilter] = useState("");

  const registered = new Map(provider.models.map((m) => [m.name, m]));
  const shown = names.filter((n) => n.toLowerCase().includes(filter.trim().toLowerCase()));
  const adding = create.isPending ? create.variables.body.name : null;

  function add(name: string, capability: Capability) {
    create.mutate(
      { providerId: provider.id, body: { name, capability } },
      {
        onSuccess: () => toast.success(t("ai.found.added", { name })),
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
          placeholder={t("ai.found.filter")}
          aria-label={t("ai.found.filter")}
          className="h-9"
        />
      )}
      <ul className="flex max-h-72 flex-col divide-y divide-border overflow-y-auto rounded-lg border border-border">
        {shown.map((name) => {
          const model = registered.get(name);
          const embedding = /embed/i.test(name);
          return (
            <li key={name} className="flex flex-wrap items-center justify-between gap-2 px-3 py-2">
              <span className="min-w-0 truncate font-mono text-[13px]" title={name}>
                {name}
              </span>
              {model ? (
                <Badge variant="outline" className="gap-1">
                  <Check aria-hidden />
                  {t("ai.found.registered", { capability: t(`ai.capability.${model.capability}`) })}
                </Badge>
              ) : (
                <span className="flex gap-1.5">
                  <Button
                    size="sm"
                    variant={embedding ? "outline" : "default"}
                    disabled={adding !== null}
                    onClick={() => add(name, "chat")}
                    className="pointer-coarse:h-11"
                    aria-label={t("ai.found.addAs", { name, capability: t("ai.capability.chat") })}
                  >
                    {t("ai.capability.chat")}
                  </Button>
                  {supportsEmbeddings(provider) && (
                    <Button
                      size="sm"
                      variant={embedding ? "default" : "outline"}
                      disabled={adding !== null}
                      onClick={() => add(name, "embedding")}
                      className="pointer-coarse:h-11"
                      aria-label={t("ai.found.addAs", {
                        name,
                        capability: t("ai.capability.embedding"),
                      })}
                    >
                      {t("ai.capability.embedding")}
                    </Button>
                  )}
                </span>
              )}
            </li>
          );
        })}
        {shown.length === 0 && (
          <li className="px-3 py-3 text-sm text-muted-foreground">{t("ai.found.noMatch")}</li>
        )}
      </ul>
      <p className="text-xs text-subtle-foreground">{t("ai.found.hint")}</p>
    </div>
  );
}
