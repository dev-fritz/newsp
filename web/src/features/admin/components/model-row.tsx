import { Trash2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { TableCell, TableRow } from "@/components/ui/table";

import { useDeleteModel, useUpdateModel } from "../api/ai-queries";
import { useErrorToast } from "../hooks/use-error-toast";
import { useFormat } from "../hooks/use-format";
import { TOUCH_TARGET } from "../lib/touch";
import type { AiModel, InstanceSettings } from "../types";
import { StatusSwitch } from "./status-switch";

type ModelRowProps = {
  model: AiModel;
  providerName: string;
  settings: InstanceSettings | undefined;
};

export function ModelRow({ model, providerName, settings }: ModelRowProps) {
  const { t } = useTranslation("admin");
  const format = useFormat();
  const toastError = useErrorToast();
  const update = useUpdateModel();
  const remove = useDeleteModel();
  const [confirming, setConfirming] = useState(false);

  const defaults = [
    settings?.embedding_model_id === model.id && t("ai.models.defaultEmbedding"),
    settings?.default_report_model_id === model.id && t("ai.models.defaultReports"),
    settings?.default_idea_model_id === model.id && t("ai.models.defaultIdeas"),
  ].filter((x): x is string => Boolean(x));

  const name = model.display_name || model.name;

  function confirmDelete() {
    remove.mutate(model.id, {
      onSuccess: () => {
        toast.success(t("ai.toast.modelDeleted", { name }));
        setConfirming(false);
      },
      onError: (error) => {
        setConfirming(false);
        toastError(error);
      },
    });
  }

  return (
    <TableRow className={model.enabled ? undefined : "text-muted-foreground"}>
      <TableCell className="whitespace-normal">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-mono text-[13px] font-medium text-foreground">{name}</span>
          {defaults.map((d) => (
            <Badge key={d} className="bg-signal-soft text-[11px] text-signal-ink">
              {d}
            </Badge>
          ))}
        </div>
      </TableCell>
      <TableCell>{providerName}</TableCell>
      <TableCell>
        <Badge
          variant={model.capability === "chat" ? "secondary" : "outline"}
          className={model.capability === "chat" ? undefined : "border-border-strong"}
        >
          {t(`ai.capability.${model.capability}`)}
        </Badge>
      </TableCell>
      <TableCell className="text-right font-mono text-[12.5px]">
        {model.tokens_last_7d > 0 ? format.tokens(model.tokens_last_7d) : "—"}
      </TableCell>
      <TableCell>
        <div className="flex items-center justify-end gap-2">
          <StatusSwitch
            checked={model.enabled}
            disabled={update.isPending}
            onCheckedChange={(enabled) =>
              update.mutate({ id: model.id, body: { enabled } }, { onError: toastError })
            }
            aria-label={t("ai.models.enabled", { name })}
          />
          <Button
            variant="ghost"
            size="icon"
            className={`${TOUCH_TARGET} hover:text-negative`}
            onClick={() => setConfirming(true)}
            aria-label={t("ai.models.delete", { name })}
          >
            <Trash2 aria-hidden />
          </Button>
        </div>
      </TableCell>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={t("ai.models.deleteTitle", { name })}
        description={t("ai.models.deleteBody")}
        confirmLabel={t("ai.models.deleteConfirm")}
        onConfirm={confirmDelete}
        pending={remove.isPending}
      />
    </TableRow>
  );
}
