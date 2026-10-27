import { useFieldArray, type UseFormReturn } from "react-hook-form";
import { useTranslation } from "react-i18next";

import { FormField } from "@/components/common/form-field";

import { IDEA_LIMITS, type EditIdeaValues } from "../schemas/idea.schema";
import { RepeatableRow, RepeatableRows } from "./repeatable-rows";
import { TextareaField } from "./textarea-field";

/** Edit rows for the MVP plan (at least two steps). */
export function PlanRows({ form }: { form: UseFormReturn<EditIdeaValues> }) {
  const { t } = useTranslation("ideas");
  const { fields, append, remove } = useFieldArray({ control: form.control, name: "mvp_plan" });
  const errors = form.formState.errors.mvp_plan;
  return (
    <RepeatableRows
      legend={t("sections.mvp")}
      addLabel={t("edit.addStep")}
      onAdd={() => append({ period: "", title: "", description: "" })}
      canAdd={fields.length < IDEA_LIMITS.planMax}
      error={errors?.root?.message ?? errors?.message}
    >
      {fields.map((row, index) => (
        <RepeatableRow
          key={row.id}
          title={t("edit.stepRow", { n: index + 1 })}
          removeLabel={t("edit.remove")}
          onRemove={() => remove(index)}
          canRemove={fields.length > IDEA_LIMITS.planMin}
        >
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-[minmax(0,160px)_minmax(0,1fr)]">
            <FormField
              label={t("edit.stepPeriod")}
              error={errors?.[index]?.period?.message}
              {...form.register(`mvp_plan.${index}.period`)}
            />
            <FormField
              label={t("edit.stepTitle")}
              error={errors?.[index]?.title?.message}
              {...form.register(`mvp_plan.${index}.title`)}
            />
          </div>
          <TextareaField
            label={t("edit.stepDescription")}
            error={errors?.[index]?.description?.message}
            {...form.register(`mvp_plan.${index}.description`)}
          />
        </RepeatableRow>
      ))}
    </RepeatableRows>
  );
}
