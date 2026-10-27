import { useId } from "react";
import { useTranslation } from "react-i18next";

import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export type SelectOption = { value: string; label: string };

type SelectFieldProps = {
  label: string;
  value: string;
  onValueChange: (value: string) => void;
  options: SelectOption[];
  placeholder?: string;
  hint?: string;
  /** i18n key under common:fieldErrors (or a backend validation tag). */
  error?: string;
  disabled?: boolean;
  id?: string;
};

/** Labeled select with an accessible hint and error, like FormField. */
export function SelectField({
  label,
  value,
  onValueChange,
  options,
  placeholder,
  hint,
  error,
  disabled,
  id,
}: SelectFieldProps) {
  const { t } = useTranslation();
  const generated = useId();
  const selectId = id ?? generated;
  const describedBy =
    [hint && `${selectId}-hint`, error && `${selectId}-error`].filter(Boolean).join(" ") ||
    undefined;
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={selectId}>{label}</Label>
      <Select value={value} onValueChange={onValueChange} disabled={disabled}>
        <SelectTrigger
          id={selectId}
          className="h-10 w-full pointer-coarse:h-11"
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
        >
          <SelectValue placeholder={placeholder} />
        </SelectTrigger>
        <SelectContent position="popper">
          {options.map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {hint && !error && (
        <p id={`${selectId}-hint`} className="text-xs text-subtle-foreground">
          {hint}
        </p>
      )}
      {error && (
        <p id={`${selectId}-error`} className="text-xs text-negative" role="alert">
          {t(`fieldErrors.${error}`, { defaultValue: t("fieldErrors.invalid") })}
        </p>
      )}
    </div>
  );
}
