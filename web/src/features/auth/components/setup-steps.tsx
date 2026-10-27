import { Check } from "lucide-react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

const STEPS = ["account", "ai", "sources"] as const;

/** Progress of the first-run setup (account → AI provider → sources). */
export function SetupSteps({ current }: { current: 1 | 2 | 3 }) {
  const { t } = useTranslation("auth");
  return (
    <ol
      aria-label={t("setup.stepOf", { current, total: STEPS.length })}
      className="flex flex-wrap items-center gap-2 text-sm"
    >
      {STEPS.map((step, i) => {
        const n = i + 1;
        const done = n < current;
        const active = n === current;
        return (
          <li
            key={step}
            className="flex items-center gap-2"
            aria-current={active ? "step" : undefined}
          >
            {i > 0 && <span aria-hidden className="h-px w-6 bg-border-strong" />}
            <span
              aria-hidden
              className={cn(
                "inline-flex size-6 items-center justify-center rounded-full font-mono text-xs",
                done && "bg-positive text-white",
                active && "bg-foreground text-background",
                !done && !active && "border border-border-strong text-subtle-foreground",
              )}
            >
              {done ? <Check className="size-3.5" /> : n}
            </span>
            <span className={cn(active ? "font-semibold" : "text-muted-foreground")}>
              {t(`setup.steps.${step}`)}
            </span>
          </li>
        );
      })}
    </ol>
  );
}
