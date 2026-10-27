import { zodResolver } from "@hookform/resolvers/zod";
import { Controller, useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { FormField } from "@/components/common/form-field";
import { Button } from "@/components/ui/button";

import { useCreateSource, useUpdateSource } from "../api/sources-queries";
import { useErrorToast } from "../hooks/use-error-toast";
import { applyFieldErrors } from "../lib/field-errors";
import {
  emptySourceValues,
  mapSourceFieldErrors,
  secretPayload,
  sourceToValues,
  valuesToConfig,
} from "../lib/source-form";
import { DEFAULT_LIMIT, KIND_TAG, SOURCE_KINDS } from "../lib/source-kinds";
import { sourceFormSchema, type SourceFormValues } from "../schemas/source.schema";
import type { Source, SourceKind } from "../types";
import { IntervalField } from "./interval-field";
import { OptionCardGroup } from "./option-card-group";
import { SourceConfigFields } from "./source-config-fields";
import { SourceKindTag } from "./source-kind-tag";

type SourceFormProps = {
  /** Edit this source; omit to create one. */
  source?: Source;
  onDone: () => void;
  onCancel: () => void;
};

/** Create or edit a source: kind picker (create only), name, kind fields and interval. */
export function SourceForm({ source, onDone, onCancel }: SourceFormProps) {
  const { t } = useTranslation("admin");
  const toastError = useErrorToast();
  const create = useCreateSource();
  const update = useUpdateSource();
  const pending = create.isPending || update.isPending;

  const form = useForm<SourceFormValues>({
    resolver: zodResolver(sourceFormSchema),
    defaultValues: source ? sourceToValues(source) : emptySourceValues(),
  });
  const { errors } = form.formState;

  function pickKind(kind: SourceKind) {
    form.setValue("kind", kind);
    // Back to the defaults of the fields that depend on the kind (limit, modes, credential).
    const defaults = emptySourceValues(kind);
    form.setValue("limit", String(DEFAULT_LIMIT[kind]));
    form.setValue("lobstersList", defaults.lobstersList);
    form.setValue("devtoTop", defaults.devtoTop);
    form.setValue("mastodonMode", defaults.mastodonMode);
    form.setValue("bskyMode", defaults.bskyMode);
    form.setValue("bskySort", defaults.bskySort);
    form.setValue("phOrder", defaults.phOrder);
    form.setValue("secret", "");
    form.setValue("clearSecret", false);
    form.clearErrors();
  }

  async function submit(v: SourceFormValues) {
    const config = valuesToConfig(v);
    const interval_minutes = Number(v.interval);
    try {
      if (source) {
        await update.mutateAsync({
          id: source.id,
          body: { name: v.name, interval_minutes, config, ...secretPayload(v) },
        });
        toast.success(t("sources.toast.updated", { name: v.name }));
      } else {
        await create.mutateAsync({
          kind: v.kind,
          name: v.name,
          interval_minutes,
          config,
          ...secretPayload(v),
        });
        toast.success(t("sources.toast.created", { name: v.name }));
      }
      onDone();
    } catch (error) {
      if (
        !applyFieldErrors(
          error,
          (fields) => mapSourceFieldErrors(fields, v.kind),
          (f, e) => form.setError(f, e),
        )
      )
        toastError(error);
    }
  }

  return (
    <form onSubmit={form.handleSubmit(submit)} noValidate className="flex flex-col gap-5">
      {source ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <SourceKindTag kind={source.kind} />
          {t(`sources.kinds.${source.kind}`)}
        </div>
      ) : (
        <Controller
          control={form.control}
          name="kind"
          render={({ field }) => (
            <OptionCardGroup
              label={t("sources.form.kind")}
              value={field.value}
              onValueChange={(v) => {
                const kind = SOURCE_KINDS.find((k) => k === v);
                if (kind) pickKind(kind);
              }}
              minWidth={120}
              options={SOURCE_KINDS.map((k) => ({
                value: k,
                title: t(`sources.kinds.${k}`),
                tag: KIND_TAG[k],
              }))}
            />
          )}
        />
      )}
      <FormField
        label={t("sources.form.name")}
        autoComplete="off"
        error={errors.name?.message}
        {...form.register("name")}
      />
      <SourceConfigFields form={form} />
      <Controller
        control={form.control}
        name="interval"
        render={({ field, fieldState }) => (
          <IntervalField
            value={field.value}
            onChange={field.onChange}
            error={fieldState.error?.message}
          />
        )}
      />
      <div className="flex justify-end gap-2 pt-1">
        <Button type="button" variant="ghost" onClick={onCancel} className="pointer-coarse:h-11">
          {t("common:actions.cancel")}
        </Button>
        <Button type="submit" disabled={pending} className="pointer-coarse:h-11">
          {source ? t("sources.form.save") : t("sources.form.add")}
        </Button>
      </div>
    </form>
  );
}
