import { Plus, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";

type RowsProps = {
  legend: string;
  addLabel: string;
  onAdd: () => void;
  canAdd: boolean;
  /** i18n key under common:fieldErrors for an error on the whole list. */
  error?: string;
  children: React.ReactNode;
};

/** A titled group of repeatable form rows with an "add" button. */
export function RepeatableRows({ legend, addLabel, onAdd, canAdd, error, children }: RowsProps) {
  const { t } = useTranslation();
  return (
    <fieldset className="m-0 flex min-w-0 flex-col gap-3 border-0 p-0">
      <legend className="pb-1 text-sm font-medium">{legend}</legend>
      {children}
      {error && (
        <p role="alert" className="text-xs text-negative">
          {t(`fieldErrors.${error}`, { defaultValue: t("fieldErrors.invalid") })}
        </p>
      )}
      <Button
        type="button"
        variant="outline"
        className="self-start"
        onClick={onAdd}
        disabled={!canAdd}
      >
        <Plus aria-hidden />
        {addLabel}
      </Button>
    </fieldset>
  );
}

type RowProps = {
  title: string;
  removeLabel: string;
  onRemove: () => void;
  canRemove: boolean;
  children: React.ReactNode;
};

/** One bordered row of a repeatable group, with its remove button. */
export function RepeatableRow({ title, removeLabel, onRemove, canRemove, children }: RowProps) {
  return (
    <div className="flex flex-col gap-3 rounded-lg border border-border bg-muted/40 p-3.5">
      <div className="flex items-center justify-between gap-2">
        <span className="kicker">{title}</span>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label={removeLabel}
          onClick={onRemove}
          disabled={!canRemove}
          className="pointer-coarse:size-11"
        >
          <Trash2 aria-hidden />
        </Button>
      </div>
      {children}
    </div>
  );
}
