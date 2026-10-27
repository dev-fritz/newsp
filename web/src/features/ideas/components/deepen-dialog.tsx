import { zodResolver } from "@hookform/resolvers/zod";
import { Controller, useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

import type { IdeaDeepening } from "../hooks/use-idea-deepening";
import {
  DEEPEN_ASPECTS,
  deepenIdeaSchema,
  IDEA_LIMITS,
  type DeepenIdeaValues,
} from "../schemas/idea.schema";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  deepening: IdeaDeepening;
  /** The version that the new one will follow. */
  currentVersion: number;
};

/** Asks which aspects to deepen (and an extra instruction), then queues the job. */
export function DeepenDialog({ open, onOpenChange, deepening, currentVersion }: Props) {
  const { t } = useTranslation(["ideas", "common"]);
  const form = useForm<DeepenIdeaValues>({
    resolver: zodResolver(deepenIdeaSchema),
    defaultValues: { aspects: ["architecture", "mvp_plan"], instruction: "" },
  });
  const onSubmit = form.handleSubmit((values) =>
    deepening.submit(values, () => {
      onOpenChange(false);
      form.reset();
    }),
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        <form onSubmit={onSubmit} noValidate className="flex flex-col gap-5">
          <DialogHeader>
            <DialogTitle className="font-serif text-2xl font-medium">
              {t("deepen.title")}
            </DialogTitle>
            <DialogDescription>
              {t("deepen.description", {
                next: currentVersion + 1,
                current: currentVersion,
              })}
            </DialogDescription>
          </DialogHeader>
          <fieldset className="m-0 flex flex-col gap-2.5 border-0 p-0">
            <legend className="pb-2 text-[13px] font-medium">{t("deepen.aspects")}</legend>
            <Controller
              control={form.control}
              name="aspects"
              render={({ field }) => (
                <>
                  {DEEPEN_ASPECTS.map((aspect) => {
                    const id = `deepen-${aspect}`;
                    const checked = field.value.includes(aspect);
                    return (
                      <div
                        key={aspect}
                        className="flex min-h-6 items-center gap-2.5 pointer-coarse:min-h-11"
                      >
                        <Checkbox
                          id={id}
                          checked={checked}
                          onCheckedChange={(next) =>
                            field.onChange(
                              next === true
                                ? [...field.value, aspect]
                                : field.value.filter((a) => a !== aspect),
                            )
                          }
                        />
                        <Label htmlFor={id} className="text-sm font-normal">
                          {t(`deepen.aspect.${aspect}`)}
                        </Label>
                      </div>
                    );
                  })}
                </>
              )}
            />
            <p className="text-xs text-subtle-foreground">{t("deepen.aspectsHint")}</p>
          </fieldset>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="deepen-instruction">{t("deepen.instruction")}</Label>
            <Textarea
              id="deepen-instruction"
              maxLength={IDEA_LIMITS.instruction}
              placeholder={t("deepen.instructionPlaceholder")}
              aria-invalid={form.formState.errors.instruction ? true : undefined}
              {...form.register("instruction")}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              {t("common:actions.cancel")}
            </Button>
            <Button type="submit" disabled={deepening.isStarting || deepening.running}>
              {deepening.isStarting
                ? t("deepen.starting")
                : t("deepen.submit", { next: currentVersion + 1 })}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
