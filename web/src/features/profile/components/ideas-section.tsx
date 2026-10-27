import { Minus, Plus } from "lucide-react";
import { Controller, useFormContext } from "react-hook-form";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { SettingsSection } from "@/features/profile/components/settings-section";
import type { PreferencesValues } from "@/features/profile/schemas/profile.schema";

const MAX_IDEAS = 10;

export function IdeasSection() {
  const { t } = useTranslation("profile");
  const { control } = useFormContext<PreferencesValues>();
  return (
    <SettingsSection id="s-ideas" title={t("ideas.title")} hint={t("ideas.hint")}>
      <div className="flex flex-col gap-1.5">
        <Label id="lbl-qty">{t("ideas.perDay")}</Label>
        <Controller
          control={control}
          name="ideas_per_day"
          render={({ field }) => (
            <fieldset
              aria-labelledby="lbl-qty"
              className="m-0 inline-flex items-center self-start rounded-[10px] border border-border-strong bg-card"
            >
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-10 pointer-coarse:size-11"
                aria-label={t("ideas.decrease")}
                disabled={field.value <= 0}
                onClick={() => field.onChange(Math.max(0, field.value - 1))}
              >
                <Minus aria-hidden />
              </Button>
              <span
                className="min-w-11 text-center font-mono text-base font-semibold"
                aria-live="polite"
              >
                {field.value}
              </span>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-10 pointer-coarse:size-11"
                aria-label={t("ideas.increase")}
                disabled={field.value >= MAX_IDEAS}
                onClick={() => field.onChange(Math.min(MAX_IDEAS, field.value + 1))}
              >
                <Plus aria-hidden />
              </Button>
            </fieldset>
          )}
        />
        <p className="text-xs text-subtle-foreground">{t("ideas.range")}</p>
      </div>
    </SettingsSection>
  );
}
