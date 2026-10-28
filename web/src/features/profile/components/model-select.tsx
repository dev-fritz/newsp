import { useTranslation } from "react-i18next";

import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { ChatModel } from "@/features/profile/api/profile-api";

/** Radix Select forbids an empty item value, so "instance default" maps to this. */
const DEFAULT_ITEM = "__default__";

type ModelSelectProps = {
  id: string;
  label: string;
  models: ChatModel[];
  /** "" means the instance default. */
  value: string;
  onChange: (id: string) => void;
  /** Backend validation tag (e.g. "unavailable"), translated under common:fieldErrors. */
  error?: string;
};

export function ModelSelect({ id, label, models, value, onChange, error }: ModelSelectProps) {
  const { t } = useTranslation(["profile", "common"]);
  // A saved id missing from the list (own model with BYOK off, removed model) stays the current
  // value, shown as unavailable, so picking another option (or the default) is a real change.
  const missing = value !== "" && !models.some((m) => m.id === value);
  const errorId = `${id}-error`;
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <Select
        value={value === "" ? DEFAULT_ITEM : value}
        onValueChange={(next) => onChange(next === DEFAULT_ITEM ? "" : next)}
      >
        <SelectTrigger
          id={id}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? errorId : undefined}
          className="h-10 w-full pointer-coarse:h-11"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={DEFAULT_ITEM}>{t("models.default")}</SelectItem>
          {missing && (
            <SelectItem value={value} className="text-muted-foreground">
              {t("models.unavailable")}
            </SelectItem>
          )}
          {models.flatMap((m) =>
            m.id
              ? [
                  <SelectItem key={m.id} value={m.id}>
                    {m.display_name ?? m.name} · {m.provider_name}
                    {m.local ? ` (${t("models.local")})` : ""}
                    {m.own ? ` (${t("models.own")})` : ""}
                  </SelectItem>,
                ]
              : [],
          )}
        </SelectContent>
      </Select>
      {error && (
        <p id={errorId} className="text-xs text-negative" role="alert">
          {t(`common:fieldErrors.${error}`, { defaultValue: t("common:fieldErrors.invalid") })}
        </p>
      )}
    </div>
  );
}
