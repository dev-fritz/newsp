import { Check, Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";

import { useCreateSource } from "../api/sources-queries";
import { useErrorToast } from "../hooks/use-error-toast";
import type { SourcePreset } from "../lib/source-presets";
import { SourceKindTag } from "./source-kind-tag";

type SourcePresetCardProps = { preset: SourcePreset; added: boolean };

/** A suggested source that is added with one click. */
export function SourcePresetCard({ preset, added }: SourcePresetCardProps) {
  const { t } = useTranslation("admin");
  const toastError = useErrorToast();
  const create = useCreateSource();

  function add() {
    create.mutate(
      {
        kind: preset.kind,
        name: preset.name,
        interval_minutes: preset.intervalMinutes,
        config: preset.config,
      },
      {
        onSuccess: () => toast.success(t("sources.toast.created", { name: preset.name })),
        onError: toastError,
      },
    );
  }

  return (
    <li className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-border bg-card p-3.5">
      <div className="flex min-w-0 items-center gap-3">
        <SourceKindTag kind={preset.kind} />
        <div className="flex min-w-0 flex-col">
          <span className="font-medium">{preset.name}</span>
          <span className="text-xs text-subtle-foreground">
            {t(`setup.sources.presets.${preset.id}`)}
          </span>
        </div>
      </div>
      {added ? (
        <span className="flex items-center gap-1.5 text-sm font-medium text-positive">
          <Check className="size-4" aria-hidden />
          {t("setup.sources.added")}
        </span>
      ) : (
        <Button
          variant="secondary"
          size="sm"
          onClick={add}
          disabled={create.isPending}
          className="pointer-coarse:h-11"
          aria-label={t("setup.sources.addNamed", { name: preset.name })}
        >
          <Plus aria-hidden />
          {t("setup.sources.add")}
        </Button>
      )}
    </li>
  );
}
