import { useFieldArray, type UseFormReturn } from "react-hook-form";
import { useTranslation } from "react-i18next";

import { FormField } from "@/components/common/form-field";

import { IDEA_LIMITS, type EditIdeaValues } from "../schemas/idea.schema";
import { RepeatableRow, RepeatableRows } from "./repeatable-rows";
import { TextareaField } from "./textarea-field";

/** Edit rows for competitors and the gap each leaves open. */
export function CompetitorRows({ form }: { form: UseFormReturn<EditIdeaValues> }) {
  const { t } = useTranslation("ideas");
  const { fields, append, remove } = useFieldArray({ control: form.control, name: "competitors" });
  const errors = form.formState.errors.competitors;
  return (
    <RepeatableRows
      legend={t("sections.competitors")}
      addLabel={t("edit.addCompetitor")}
      onAdd={() => append({ name: "", strengths: "", gap: "" })}
      canAdd={fields.length < IDEA_LIMITS.competitorsMax}
      error={errors?.root?.message ?? errors?.message}
    >
      {fields.map((row, index) => (
        <RepeatableRow
          key={row.id}
          title={t("edit.competitorRow", { n: index + 1 })}
          removeLabel={t("edit.remove")}
          onRemove={() => remove(index)}
          canRemove
        >
          <FormField
            label={t("competitors.name")}
            error={errors?.[index]?.name?.message}
            {...form.register(`competitors.${index}.name`)}
          />
          <TextareaField
            label={t("competitors.strengths")}
            error={errors?.[index]?.strengths?.message}
            {...form.register(`competitors.${index}.strengths`)}
          />
          <TextareaField
            label={t("competitors.gap")}
            error={errors?.[index]?.gap?.message}
            {...form.register(`competitors.${index}.gap`)}
          />
        </RepeatableRow>
      ))}
    </RepeatableRows>
  );
}
