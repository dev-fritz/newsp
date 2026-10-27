import { zodResolver } from "@hookform/resolvers/zod";
import { Sparkles } from "lucide-react";
import { Controller, useForm, useWatch } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { FormField } from "@/components/common/form-field";
import { Button } from "@/components/ui/button";
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import type { ApiError } from "@/lib/api-error";

import type { GenerateIdeasRequest } from "../api/ideas-api";
import { useErrorMessage } from "../hooks/use-error-message";
import {
  useIdeaGeneration,
  type GenerationBase,
  type ReportBase,
} from "../hooks/use-idea-generation";
import { generateIdeasSchema, IDEA_LIMITS, type GenerateIdeasValues } from "../schemas/idea.schema";
import { ItemPicker } from "./item-picker";
import { Segmented } from "./segmented";

const COUNTS = ["1", "3", "5"] as const;
const WINDOWS = ["1", "7", "30"] as const;
const DEFAULTS: GenerateIdeasValues = { focus: "", count: 3, window_days: 7, items: [] };

/** Form fields the API may flag, mapped to the form's own. */
const FIELDS = {
  focus: "focus",
  count: "count",
  window_days: "window_days",
  item_ids: "items",
} as const;

function isApiField(field: string): field is keyof typeof FIELDS {
  return field in FIELDS;
}

/** Count, window, focus and picked items; from a report, only the count. */
export function GenerateIdeasForm({ report }: { report: ReportBase | null }) {
  const { t } = useTranslation(["ideas", "common"]);
  const errorMessage = useErrorMessage();
  const generation = useIdeaGeneration();
  const form = useForm<GenerateIdeasValues>({
    resolver: zodResolver(generateIdeasSchema),
    defaultValues: generation.draft ?? DEFAULTS,
  });
  const { errors } = form.formState;
  const count = useWatch({ control: form.control, name: "count" });
  const windowDays = useWatch({ control: form.control, name: "window_days" });
  const fieldError = (tag: string | undefined) =>
    t([
      `ideas:dialog.fieldErrors.${tag}`,
      `common:fieldErrors.${tag}`,
      "common:fieldErrors.invalid",
    ]);

  function onError(error: ApiError) {
    if (error.fields?.report_id === "not_ready") {
      toast.error(t("ideas:errors.REPORT_NOT_READY"));
      return;
    }
    // List fields come back per element too ("item_ids[0]").
    const flagged = Object.entries(error.fields ?? {})
      .map(([field, tag]) => [field.replace(/\[\d+\]$/, ""), tag] as const)
      .filter(([field]) => isApiField(field));
    for (const [field, tag] of flagged) {
      if (isApiField(field)) form.setError(FIELDS[field], { message: tag });
    }
    if (flagged.length === 0) toast.error(errorMessage(error.code));
  }

  const onSubmit = form.handleSubmit((values) => {
    if (report) {
      const request: GenerateIdeasRequest = { count: values.count, report_id: report.id };
      generation.start(request, { kind: "report", report }, generation.draft, onError);
      return;
    }
    const request: GenerateIdeasRequest = {
      count: values.count,
      window_days: values.window_days,
      focus: values.focus || undefined,
      item_ids: values.items.length > 0 ? values.items.map((item) => item.id) : undefined,
    };
    const base: GenerationBase = {
      kind: "window",
      windowDays: values.window_days,
      focus: values.focus,
      items: values.items.length,
    };
    generation.start(request, base, values, onError);
  });

  return (
    <form onSubmit={onSubmit} noValidate className="flex flex-col gap-5">
      <DialogHeader className="pr-8">
        <DialogTitle className="font-serif text-[26px] leading-tight font-medium">
          {report ? t("ideas:dialog.reportTitle") : t("ideas:dialog.title")}
        </DialogTitle>
        <DialogDescription>
          {report ? t("ideas:dialog.reportDescription") : t("ideas:dialog.description")}
        </DialogDescription>
      </DialogHeader>

      {report ? (
        <div className="flex flex-col gap-1.5 rounded-lg bg-muted px-4 py-3.5">
          <span className="kicker">{t("ideas:dialog.base")}</span>
          <span className="font-serif text-[19px] leading-snug font-medium text-balance">
            {report.title}
          </span>
          <span className="font-mono text-[11.5px] text-subtle-foreground">{report.meta}</span>
        </div>
      ) : (
        <FormField
          label={t("ideas:dialog.focus")}
          placeholder={t("ideas:dialog.focusPlaceholder")}
          autoComplete="off"
          maxLength={IDEA_LIMITS.focus}
          error={errors.focus?.message}
          {...form.register("focus")}
        />
      )}

      <div className="flex flex-wrap gap-4">
        <div className="flex flex-col gap-1.5">
          <span aria-hidden className="text-sm font-medium">
            {t("ideas:generate.count")}
          </span>
          <Controller
            control={form.control}
            name="count"
            render={({ field }) => (
              <Segmented
                label={t("ideas:generate.count")}
                value={String(field.value)}
                options={COUNTS.map((value) => ({ value, label: value }))}
                onChange={(value) => field.onChange(Number(value))}
              />
            )}
          />
        </div>
        {!report && (
          <div className="flex flex-col gap-1.5">
            <span aria-hidden className="text-sm font-medium">
              {t("ideas:generate.window")}
            </span>
            <Controller
              control={form.control}
              name="window_days"
              render={({ field }) => (
                <Segmented
                  label={t("ideas:generate.window")}
                  value={String(field.value)}
                  options={WINDOWS.map((value) => ({
                    value,
                    label: t(`ideas:dialog.window.d${value}`),
                  }))}
                  onChange={(value) => field.onChange(Number(value))}
                />
              )}
            />
          </div>
        )}
      </div>

      {!report && (
        <div className="flex flex-col gap-1.5">
          <span id="generate-items-label" className="text-sm font-medium">
            {t("ideas:dialog.items")}
          </span>
          <Controller
            control={form.control}
            name="items"
            render={({ field }) => (
              <ItemPicker
                value={field.value}
                onChange={field.onChange}
                windowDays={windowDays}
                max={IDEA_LIMITS.items}
                labelledBy="generate-items-label"
                error={errors.items ? fieldError(errors.items.message) : undefined}
              />
            )}
          />
        </div>
      )}

      <DialogFooter className="border-t border-border pt-4">
        <Button
          type="button"
          variant="ghost"
          className="h-9 pointer-coarse:h-11"
          onClick={generation.closeDialog}
        >
          {t("common:actions.cancel")}
        </Button>
        <Button type="submit" className="h-9 pointer-coarse:h-11" disabled={generation.isStarting}>
          <Sparkles aria-hidden />
          {generation.isStarting ? t("ideas:dialog.starting") : t("ideas:dialog.submit", { count })}
        </Button>
      </DialogFooter>
    </form>
  );
}
