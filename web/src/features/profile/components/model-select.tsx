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
};

export function ModelSelect({ id, label, models, value, onChange }: ModelSelectProps) {
  const { t } = useTranslation("profile");
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <Select
        value={value || DEFAULT_ITEM}
        onValueChange={(next) => onChange(next === DEFAULT_ITEM ? "" : next)}
      >
        <SelectTrigger id={id} className="h-10 w-full pointer-coarse:h-11">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={DEFAULT_ITEM}>{t("models.default")}</SelectItem>
          {models.flatMap((m) =>
            m.id
              ? [
                  <SelectItem key={m.id} value={m.id}>
                    {m.display_name ?? m.name} · {m.provider_name}
                    {m.local ? ` (${t("models.local")})` : ""}
                  </SelectItem>,
                ]
              : [],
          )}
        </SelectContent>
      </Select>
    </div>
  );
}
