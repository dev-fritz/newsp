import { Controller, useFormContext } from "react-hook-form";
import { useTranslation } from "react-i18next";

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { SegmentedControl } from "@/features/profile/components/segmented-control";
import { SettingsSection } from "@/features/profile/components/settings-section";
import { TimezoneSelect } from "@/features/profile/components/timezone-select";
import type { PreferencesValues } from "@/features/profile/schemas/profile.schema";

type ReportSectionProps = { nextReportAt?: string; savedTimezone?: string };

export function ReportSection({ nextReportAt, savedTimezone }: ReportSectionProps) {
  const { t, i18n } = useTranslation("profile");
  const { control, register } = useFormContext<PreferencesValues>();
  const next = formatNext(nextReportAt, savedTimezone, i18n.language);
  return (
    <SettingsSection id="s-report" title={t("report.title")} hint={t("report.hint")}>
      <div className="flex items-center justify-between gap-3">
        <Label id="lbl-report">{t("report.enabled")}</Label>
        <Controller
          control={control}
          name="report_enabled"
          render={({ field }) => (
            <Switch
              aria-labelledby="lbl-report"
              checked={field.value}
              onCheckedChange={field.onChange}
            />
          )}
        />
      </div>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(180px,100%),1fr))] gap-3.5">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="pref-time">{t("report.time")}</Label>
          <Input id="pref-time" type="time" className="h-10" {...register("report_time")} />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="pref-tz">{t("report.timezone")}</Label>
          <Controller
            control={control}
            name="timezone"
            render={({ field }) => (
              <TimezoneSelect id="pref-tz" value={field.value} onChange={field.onChange} />
            )}
          />
        </div>
      </div>
      {next && (
        <p className="text-sm text-muted-foreground" aria-live="polite">
          {t("report.next", { when: next })}
        </p>
      )}
      <div className="flex flex-col gap-1.5">
        <Label id="lbl-lang">{t("report.outputLang")}</Label>
        <Controller
          control={control}
          name="output_lang"
          render={({ field }) => (
            <SegmentedControl
              labelledBy="lbl-lang"
              value={field.value}
              onChange={field.onChange}
              options={[
                { value: "pt-BR", label: t("report.pt-BR") },
                { value: "en", label: t("report.en") },
              ]}
            />
          )}
        />
        <p className="text-xs text-subtle-foreground">{t("report.outputLangHint")}</p>
      </div>
    </SettingsSection>
  );
}

/** The next run in the zone the preferences were saved with. */
function formatNext(iso: string | undefined, timeZone: string | undefined, locale: string): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  try {
    return new Intl.DateTimeFormat(locale, {
      dateStyle: "full",
      timeStyle: "short",
      timeZone,
    }).format(date);
  } catch {
    return new Intl.DateTimeFormat(locale, { dateStyle: "full", timeStyle: "short" }).format(date);
  }
}
