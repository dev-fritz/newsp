import { Controller, useFieldArray, type UseFormReturn } from "react-hook-form";
import { useTranslation } from "react-i18next";

import { FormField } from "@/components/common/form-field";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

import { audienceKindSchema } from "../schemas/idea-response.schema";
import { IDEA_LIMITS, type EditIdeaValues } from "../schemas/idea.schema";
import { RepeatableRow, RepeatableRows } from "./repeatable-rows";
import { TextareaField } from "./textarea-field";

/** Edit rows for the audience segments (who it is for, and who it is not). */
export function AudienceRows({ form }: { form: UseFormReturn<EditIdeaValues> }) {
  const { t } = useTranslation("ideas");
  const { fields, append, remove } = useFieldArray({ control: form.control, name: "audience" });
  const errors = form.formState.errors.audience;
  return (
    <RepeatableRows
      legend={t("sections.audience")}
      addLabel={t("edit.addAudience")}
      onAdd={() => append({ kind: "secondary", label: "", description: "" })}
      canAdd={fields.length < IDEA_LIMITS.audienceMax}
      error={errors?.root?.message ?? errors?.message}
    >
      {fields.map((row, index) => (
        <RepeatableRow
          key={row.id}
          title={t("edit.audienceRow", { n: index + 1 })}
          removeLabel={t("edit.remove")}
          onRemove={() => remove(index)}
          canRemove={fields.length > 1}
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor={`audience-${row.id}-kind`}>{t("edit.audienceKind")}</Label>
            <Controller
              control={form.control}
              name={`audience.${index}.kind`}
              render={({ field }) => (
                <Select value={field.value} onValueChange={field.onChange}>
                  <SelectTrigger id={`audience-${row.id}-kind`} className="h-10 w-full sm:w-64">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {audienceKindSchema.options.map((kind) => (
                      <SelectItem key={kind} value={kind}>
                        {t(`audience.${kind}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            />
          </div>
          <FormField
            label={t("edit.audienceLabel")}
            error={errors?.[index]?.label?.message}
            {...form.register(`audience.${index}.label`)}
          />
          <TextareaField
            label={t("edit.audienceDescription")}
            error={errors?.[index]?.description?.message}
            {...form.register(`audience.${index}.description`)}
          />
        </RepeatableRow>
      ))}
    </RepeatableRows>
  );
}
