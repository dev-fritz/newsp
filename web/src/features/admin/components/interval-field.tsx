import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

import { useFormat } from "../hooks/use-format";
import { INTERVAL_PRESETS, MAX_INTERVAL, MIN_INTERVAL } from "../lib/source-kinds";
import { SelectField } from "./select-field";

const CUSTOM = "custom";

type IntervalFieldProps = {
  /** Minutes, as text. */
  value: string;
  onChange: (value: string) => void;
  error?: string;
};

/** Collection interval: common presets plus a custom number of minutes (5 to 1440). */
export function IntervalField({ value, onChange, error }: IntervalFieldProps) {
  const { t } = useTranslation("admin");
  const format = useFormat();
  const inputId = useId();
  const [custom, setCustom] = useState(!(INTERVAL_PRESETS as readonly string[]).includes(value));

  const options = [
    ...INTERVAL_PRESETS.map((m) => ({ value: m, label: format.interval(Number(m)) })),
    { value: CUSTOM, label: t("sources.form.intervalCustom") },
  ];

  return (
    <div className="flex flex-col gap-3">
      <SelectField
        label={t("sources.form.interval")}
        value={custom ? CUSTOM : value}
        onValueChange={(v) => {
          if (v === CUSTOM) setCustom(true);
          else {
            setCustom(false);
            onChange(v);
          }
        }}
        options={options}
        error={custom ? undefined : error}
      />
      {custom && (
        <div className="flex flex-col gap-1.5">
          <Label htmlFor={inputId}>{t("sources.form.intervalMinutes")}</Label>
          <div className="flex items-center gap-2">
            <Input
              id={inputId}
              type="number"
              inputMode="numeric"
              min={MIN_INTERVAL}
              max={MAX_INTERVAL}
              value={value}
              onChange={(e) => onChange(e.target.value)}
              aria-invalid={error ? true : undefined}
              aria-describedby={`${inputId}-hint`}
              className="h-10 w-28"
            />
            <span className="text-sm text-muted-foreground">{t("sources.form.minutes")}</span>
          </div>
          <p
            id={`${inputId}-hint`}
            className={error ? "text-xs text-negative" : "text-xs text-subtle-foreground"}
            role={error ? "alert" : undefined}
          >
            {t("sources.form.intervalHint", { min: MIN_INTERVAL, max: MAX_INTERVAL })}
          </p>
        </div>
      )}
    </div>
  );
}
