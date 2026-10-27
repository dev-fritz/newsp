import { useTranslation } from "react-i18next";

import { DiffText } from "./diff-text";

/** Key to the diff marks. */
export function DiffLegend() {
  const { t } = useTranslation("ideas");
  return (
    <p className="m-0 flex flex-wrap items-center gap-4 text-[13px]">
      <span className="text-muted-foreground">{t("compare.legend")}</span>
      <span>
        <DiffText ops={[{ type: "add", text: t("compare.legendAdded") }]} />
      </span>
      <span>
        <DiffText ops={[{ type: "remove", text: t("compare.legendRemoved") }]} />
      </span>
    </p>
  );
}
