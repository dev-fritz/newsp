import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { useCreateReport } from "../api/reports-queries";
import { generateReportSchema, type GenerateReportValues } from "../schemas/generate-report.schema";
import { WINDOW_CHOICES } from "../types";

type GenerateReportDialogProps = {
  /** The element that opens the dialog (rendered with `asChild`). */
  children: React.ReactNode;
  /** Pre-fills the form, e.g. to retry a failed report with the same window and focus. */
  defaults?: { windowHours?: number; focus?: string };
  onGenerated?: () => void;
};

function initialWindow(hours: number | undefined): GenerateReportValues["window_hours"] {
  const match = WINDOW_CHOICES.find((choice) => choice === hours);
  return String(match ?? 24) as GenerateReportValues["window_hours"];
}

/** "Gerar agora": window choices plus an optional focus, queued as a manual report. */
export function GenerateReportDialog({
  children,
  defaults,
  onGenerated,
}: GenerateReportDialogProps) {
  const { t } = useTranslation("reports");
  const [open, setOpen] = useState(false);
  const create = useCreateReport();
  const form = useForm<GenerateReportValues>({
    resolver: zodResolver(generateReportSchema),
    defaultValues: {
      window_hours: initialWindow(defaults?.windowHours),
      focus: defaults?.focus ?? "",
    },
  });
  const { errors } = form.formState;

  const onSubmit = form.handleSubmit((values) => {
    create.mutate(
      { window_hours: Number(values.window_hours), focus: values.focus || undefined },
      {
        onSuccess: () => {
          toast.success(t("generate.success"));
          setOpen(false);
          onGenerated?.();
        },
        onError: (error) => {
          for (const [field, tag] of Object.entries(error.fields ?? {})) {
            if (field === "focus" || field === "window_hours") {
              form.setError(field, { message: tag });
            }
          }
          toast.error(t([`errors.${error.code}`, `common:errors.${error.code}`, "errors.generic"]));
        },
      },
    );
  });

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>{children}</DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={onSubmit} noValidate className="grid gap-5">
          <DialogHeader>
            <DialogTitle className="font-serif text-2xl font-medium">
              {t("generate.title")}
            </DialogTitle>
            <DialogDescription>{t("generate.description")}</DialogDescription>
          </DialogHeader>

          <div className="flex flex-col gap-1.5">
            <Label id="report-window-label">{t("generate.windowLabel")}</Label>
            <Controller
              control={form.control}
              name="window_hours"
              render={({ field }) => (
                <ToggleGroup
                  type="single"
                  variant="outline"
                  value={field.value}
                  onValueChange={(value) => {
                    if (value) field.onChange(value);
                  }}
                  aria-labelledby="report-window-label"
                  className="grid w-full grid-cols-2 gap-2"
                >
                  {WINDOW_CHOICES.map((hours) => (
                    <ToggleGroupItem
                      key={hours}
                      value={String(hours)}
                      className="h-10 w-full pointer-coarse:h-11"
                    >
                      {t(`generate.windows.${hours}`)}
                    </ToggleGroupItem>
                  ))}
                </ToggleGroup>
              )}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="report-focus">{t("generate.focusLabel")}</Label>
            <Textarea
              id="report-focus"
              rows={2}
              maxLength={200}
              placeholder={t("generate.focusPlaceholder")}
              aria-invalid={errors.focus ? true : undefined}
              aria-describedby={errors.focus ? "report-focus-error" : "report-focus-hint"}
              {...form.register("focus")}
            />
            {errors.focus ? (
              <p id="report-focus-error" role="alert" className="text-xs text-negative">
                {t(`common:fieldErrors.${errors.focus.message}`, {
                  defaultValue: t("common:fieldErrors.invalid"),
                })}
              </p>
            ) : (
              <p id="report-focus-hint" className="text-xs text-subtle-foreground">
                {t("generate.focusHint")}
              </p>
            )}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              className="h-9 pointer-coarse:h-11"
              onClick={() => setOpen(false)}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button type="submit" className="h-9 pointer-coarse:h-11" disabled={create.isPending}>
              {create.isPending ? t("generate.submitting") : t("generate.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
