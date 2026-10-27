import { Monitor, Moon, Sun } from "lucide-react";
import { Controller, useFormContext } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { useFetcher } from "react-router";

import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { SegmentedControl } from "@/features/profile/components/segmented-control";
import { SettingsSection } from "@/features/profile/components/settings-section";
import type { PreferencesValues } from "@/features/profile/schemas/profile.schema";
import { applyTheme, type Theme } from "@/lib/theme";

export function AppearanceSection() {
  const { t } = useTranslation("profile");
  const { control } = useFormContext<PreferencesValues>();
  const themeFetcher = useFetcher();

  // The cookie keeps the theme for SSR; saving the form persists it in the account.
  function changeTheme(theme: Theme, onChange: (theme: Theme) => void): void {
    onChange(theme);
    applyTheme(theme);
    void themeFetcher.submit({ theme }, { method: "post", action: "/theme" });
  }

  return (
    <SettingsSection id="s-look" title={t("appearance.title")} hint={t("appearance.hint")}>
      <div className="flex flex-col gap-1.5">
        <Label id="lbl-theme">{t("appearance.theme")}</Label>
        <Controller
          control={control}
          name="theme"
          render={({ field }) => (
            <SegmentedControl
              labelledBy="lbl-theme"
              value={field.value}
              onChange={(theme) => changeTheme(theme, field.onChange)}
              options={[
                { value: "light", label: t("appearance.light"), icon: <Sun aria-hidden /> },
                { value: "dark", label: t("appearance.dark"), icon: <Moon aria-hidden /> },
                { value: "system", label: t("appearance.system"), icon: <Monitor aria-hidden /> },
              ]}
            />
          )}
        />
      </div>
      <div className="flex max-w-70 flex-col gap-1.5">
        <Label htmlFor="pref-ui-locale">{t("appearance.uiLocale")}</Label>
        <Controller
          control={control}
          name="ui_locale"
          render={({ field }) => (
            <Select value={field.value} onValueChange={field.onChange}>
              <SelectTrigger id="pref-ui-locale" className="h-10 w-full pointer-coarse:h-11">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="pt-BR">{t("appearance.pt-BR")}</SelectItem>
                <SelectItem value="en">{t("appearance.en")}</SelectItem>
              </SelectContent>
            </Select>
          )}
        />
      </div>
    </SettingsSection>
  );
}
