import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";

type UnsavedBarProps = {
  formId: string;
  count: number;
  saving: boolean;
  onDiscard: () => void;
};

/** Sticky bar shown while the form has unsaved changes. */
export function UnsavedBar({ formId, count, saving, onDiscard }: UnsavedBarProps) {
  const { t } = useTranslation("profile");
  if (count === 0) return null;
  return (
    <section
      aria-label={t("bar.label")}
      className="sticky bottom-5 z-10 mt-6 flex flex-wrap items-center justify-between gap-3 rounded-2xl bg-foreground py-3 pr-3.5 pl-5 text-background shadow-lg"
    >
      <span className="text-sm" aria-live="polite">
        {t("bar.unsaved", { count })}
      </span>
      <span className="flex gap-2">
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="text-background hover:bg-background/10 hover:text-background pointer-coarse:h-11"
          disabled={saving}
          onClick={onDiscard}
        >
          {t("bar.discard")}
        </Button>
        <Button
          type="submit"
          form={formId}
          size="sm"
          className="bg-background text-foreground hover:bg-background/90 pointer-coarse:h-11"
          disabled={saving}
        >
          {saving ? t("bar.saving") : t("bar.save")}
        </Button>
      </span>
    </section>
  );
}
