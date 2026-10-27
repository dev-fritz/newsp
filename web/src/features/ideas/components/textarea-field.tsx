import { useId } from "react";
import { useTranslation } from "react-i18next";

import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

type TextareaFieldProps = React.ComponentProps<typeof Textarea> & {
  label: string;
  hint?: string;
  /** i18n key under common:fieldErrors (or a backend validation tag). */
  error?: string;
};

/** Labeled textarea with an accessible hint and error, like FormField for long text. */
export function TextareaField({ label, hint, error, id, className, ...props }: TextareaFieldProps) {
  const { t } = useTranslation();
  const generated = useId();
  const fieldId = id ?? generated;
  const describedBy =
    [hint && `${fieldId}-hint`, error && `${fieldId}-error`].filter(Boolean).join(" ") || undefined;
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={fieldId}>{label}</Label>
      <Textarea
        id={fieldId}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        className={className}
        {...props}
      />
      {hint && !error && (
        <p id={`${fieldId}-hint`} className="text-xs text-subtle-foreground">
          {hint}
        </p>
      )}
      {error && (
        <p id={`${fieldId}-error`} className="text-xs text-negative" role="alert">
          {t(`fieldErrors.${error}`, { defaultValue: t("fieldErrors.invalid") })}
        </p>
      )}
    </div>
  );
}
