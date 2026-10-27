import { Pencil, Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";

type Props = {
  onEdit: () => void;
  onDeepen: () => void;
  deepening: boolean;
  /** Large touch targets for the mobile bar. */
  large?: boolean;
};

/** "Edit" and "Deepen" buttons; deepening is disabled while its job runs. */
export function IdeaActions({ onEdit, onDeepen, deepening, large }: Props) {
  const { t } = useTranslation("ideas");
  const size = large ? "h-12 text-sm" : "h-8";
  return (
    <>
      <Button type="button" variant="secondary" className={size} onClick={onEdit}>
        <Pencil aria-hidden />
        {t("actions.edit")}
      </Button>
      <Button type="button" className={size} onClick={onDeepen} disabled={deepening}>
        <Sparkles aria-hidden />
        {deepening ? t("actions.deepening") : t("actions.deepen")}
      </Button>
    </>
  );
}
