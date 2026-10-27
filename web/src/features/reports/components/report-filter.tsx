import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import type { ReportKind } from "../types";

const OPTIONS: { value: ReportKind | undefined; key: "all" | "scheduled" | "manual" }[] = [
  { value: undefined, key: "all" },
  { value: "scheduled", key: "scheduled" },
  { value: "manual", key: "manual" },
];

/** Segmented filter: all / daily / on demand. */
export function ReportFilter({
  value,
  onChange,
}: {
  value: ReportKind | undefined;
  onChange: (kind: ReportKind | undefined) => void;
}) {
  const { t } = useTranslation("reports");
  return (
    <fieldset className="m-0 flex min-w-0 gap-0.5 rounded-[9px] border border-border bg-muted p-[3px]">
      <legend className="sr-only">{t("list.filterLabel")}</legend>
      {OPTIONS.map((option) => {
        const pressed = option.value === value;
        return (
          <button
            key={option.key}
            type="button"
            aria-pressed={pressed}
            onClick={() => onChange(option.value)}
            className={cn(
              "inline-flex min-h-[30px] flex-1 items-center justify-center rounded-[7px] px-3 text-[13px] font-medium text-muted-foreground transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50 pointer-coarse:min-h-11",
              pressed && "bg-card text-foreground shadow-xs",
            )}
          >
            {t(`list.filters.${option.key}`)}
          </button>
        );
      })}
    </fieldset>
  );
}
