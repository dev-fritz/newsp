import { useId } from "react";
import { useTranslation } from "react-i18next";

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

type FormFieldProps = React.ComponentProps<typeof Input> & {
  label: string;
  hint?: string;
  /** i18n key under common:fieldErrors (or a backend validation tag). */
  error?: string;
};

/** Labeled input with an accessible hint and error message. */
export function FormField({ label, hint, error, id, ...props }: FormFieldProps) {
  const { t } = useTranslation();
  const generated = useId();
  const inputId = id ?? generated;
  const describedBy =
    [hint && `${inputId}-hint`, error && `${inputId}-error`].filter(Boolean).join(" ") || undefined;
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={inputId}>{label}</Label>
      <Input
        id={inputId}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        className="h-10"
        {...props}
      />
      {hint && !error && (
        <p id={`${inputId}-hint`} className="text-xs text-subtle-foreground">
          {hint}
        </p>
      )}
      {error && (
        <p id={`${inputId}-error`} className="text-xs text-negative" role="alert">
          {t(`fieldErrors.${error}`, { defaultValue: t("fieldErrors.invalid") })}
        </p>
      )}
    </div>
  );
}
