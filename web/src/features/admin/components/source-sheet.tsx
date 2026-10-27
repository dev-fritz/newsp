import { useTranslation } from "react-i18next";

import type { Source } from "../types";
import { ResponsiveSheet } from "./responsive-sheet";
import { SourceForm } from "./source-form";

type SourceSheetProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Edit this source; omit to create one. */
  source?: Source;
};

/** Side sheet (bottom sheet on mobile) with the source form. */
export function SourceSheet({ open, onOpenChange, source }: SourceSheetProps) {
  const { t } = useTranslation("admin");
  return (
    <ResponsiveSheet
      open={open}
      onOpenChange={onOpenChange}
      title={source ? t("sources.sheet.editTitle") : t("sources.sheet.newTitle")}
      description={source ? undefined : t("sources.sheet.newDescription")}
    >
      <SourceForm
        source={source}
        onDone={() => onOpenChange(false)}
        onCancel={() => onOpenChange(false)}
      />
    </ResponsiveSheet>
  );
}
