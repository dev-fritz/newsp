import { zodResolver } from "@hookform/resolvers/zod";
import { Controller, useForm, type FieldPath } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { FormField } from "@/components/common/form-field";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

import { useEditIdea } from "../api/ideas-queries";
import { useErrorMessage } from "../hooks/use-error-message";
import { formFieldPath } from "../lib/idea-text";
import { toEditRequest, toFormValues } from "../lib/idea-form";
import { difficultySchema, type Idea } from "../schemas/idea-response.schema";
import { editIdeaSchema, IDEA_LIMITS, type EditIdeaValues } from "../schemas/idea.schema";
import { AudienceRows } from "./audience-rows";
import { CompetitorRows } from "./competitor-rows";
import { PlanRows } from "./plan-rows";
import { TextareaField } from "./textarea-field";

type Props = { idea: Idea; onDone: () => void };

/** Every editable field of the current version; saving stores a new "manual" version. */
export function EditIdeaForm({ idea, onDone }: Props) {
  const { t } = useTranslation(["ideas", "common"]);
  const errorMessage = useErrorMessage();
  const edit = useEditIdea(idea.id);
  const form = useForm<EditIdeaValues>({
    resolver: zodResolver(editIdeaSchema),
    defaultValues: toFormValues(idea.current),
  });
  const { errors } = form.formState;

  const onSubmit = form.handleSubmit((values) =>
    edit.mutate(toEditRequest(values), {
      onSuccess: (saved) => {
        toast.success(t("edit.saved", { version: saved.current_version }));
        onDone();
      },
      onError: (error) => {
        for (const [path, tag] of Object.entries(error.fields ?? {})) {
          form.setError(formFieldPath(path) as FieldPath<EditIdeaValues>, { message: tag });
        }
        toast.error(errorMessage(error.code));
      },
    }),
  );

  return (
    <form onSubmit={onSubmit} noValidate className="flex min-h-0 flex-1 flex-col">
      <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-4 pb-4">
        <FormField
          label={t("edit.title")}
          maxLength={IDEA_LIMITS.title}
          error={errors.title?.message}
          {...form.register("title")}
        />
        <TextareaField
          label={t("edit.summary")}
          rows={3}
          error={errors.summary?.message}
          {...form.register("summary")}
        />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="edit-difficulty">{t("edit.difficulty")}</Label>
            <Controller
              control={form.control}
              name="difficulty"
              render={({ field }) => (
                <Select value={field.value} onValueChange={field.onChange}>
                  <SelectTrigger id="edit-difficulty" className="h-10 w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {difficultySchema.options.map((level) => (
                      <SelectItem key={level} value={level}>
                        {t(`difficulty.${level}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            />
          </div>
          <FormField
            label={t("edit.effort")}
            type="number"
            inputMode="numeric"
            min={1}
            max={IDEA_LIMITS.effortMax}
            error={errors.effort_weeks?.message}
            {...form.register("effort_weeks", { valueAsNumber: true })}
          />
        </div>
        <TextareaField
          label={t("edit.whyNow")}
          hint={t("edit.markdownHint")}
          rows={6}
          error={errors.why_now?.message}
          {...form.register("why_now")}
        />
        <AudienceRows form={form} />
        <TextareaField
          label={t("edit.how")}
          hint={t("edit.markdownHint")}
          rows={6}
          error={errors.how?.message}
          {...form.register("how")}
        />
        <TextareaField
          label={t("edit.architecture")}
          hint={t("edit.linesHint")}
          rows={4}
          error={errors.architecture_steps?.message}
          {...form.register("architecture_steps")}
        />
        <TextareaField
          label={t("edit.stack")}
          hint={t("edit.linesHint")}
          rows={4}
          error={errors.stack?.message}
          {...form.register("stack")}
        />
        <TextareaField
          label={t("edit.differentiator")}
          rows={3}
          error={errors.differentiator?.message}
          {...form.register("differentiator")}
        />
        <PlanRows form={form} />
        <TextareaField
          label={t("edit.monetization")}
          rows={3}
          error={errors.monetization?.message}
          {...form.register("monetization")}
        />
        <CompetitorRows form={form} />
        <p className="text-xs text-subtle-foreground">{t("edit.evidenceNote")}</p>
      </div>
      <div className="flex flex-col-reverse gap-2 border-t border-border p-4 pb-[calc(1rem+env(safe-area-inset-bottom))] sm:flex-row sm:justify-end">
        <Button type="button" variant="ghost" onClick={onDone}>
          {t("common:actions.cancel")}
        </Button>
        <Button type="submit" disabled={edit.isPending}>
          {edit.isPending ? t("edit.saving") : t("edit.save")}
        </Button>
      </div>
    </form>
  );
}
