import { Controller, useFormContext } from "react-hook-form";
import { useTranslation } from "react-i18next";

import { Checkbox } from "@/components/ui/checkbox";
import type { SourceSummary } from "@/features/profile/api/profile-api";
import { SettingsSection } from "@/features/profile/components/settings-section";
import type { PreferencesValues } from "@/features/profile/schemas/profile.schema";

export function SourcesSection({ sources }: { sources: SourceSummary[] }) {
  const { t } = useTranslation("profile");
  const { control } = useFormContext<PreferencesValues>();
  return (
    <SettingsSection id="s-sources" title={t("sources.title")} hint={t("sources.hint")}>
      {sources.length === 0 ? (
        <p className="rounded-xl border border-dashed border-border-strong p-4 text-sm text-muted-foreground">
          {t("sources.empty")}
        </p>
      ) : (
        <Controller
          control={control}
          name="included_source_ids"
          render={({ field }) => (
            <ul className="flex flex-col rounded-xl border border-border bg-card">
              {sources.map((source) => {
                const id = source.id ?? "";
                const checked = field.value.includes(id);
                return (
                  <li key={id} className="border-b border-border last:border-b-0">
                    <label className="flex min-h-11 cursor-pointer items-center gap-3 px-3.5 py-3 text-sm">
                      <Checkbox
                        checked={checked}
                        onCheckedChange={(next) =>
                          field.onChange(
                            next === true
                              ? [...field.value, id]
                              : field.value.filter((v) => v !== id),
                          )
                        }
                      />
                      <span className="min-w-0 break-words">{source.name}</span>
                      {source.kind && (
                        <span className="ml-auto font-mono text-xs text-subtle-foreground">
                          {source.kind}
                        </span>
                      )}
                    </label>
                  </li>
                );
              })}
            </ul>
          )}
        />
      )}
    </SettingsSection>
  );
}
