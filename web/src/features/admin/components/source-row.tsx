import { Pencil, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { TableCell, TableRow } from "@/components/ui/table";

import { useUpdateSource } from "../api/sources-queries";
import { useErrorToast } from "../hooks/use-error-toast";
import { useFormat } from "../hooks/use-format";
import { TOUCH_TARGET } from "../lib/touch";
import type { Source } from "../types";
import { CollectSourceButton } from "./collect-source-button";
import { SourceDetail } from "./source-detail";
import { SourceKindTag } from "./source-kind-tag";
import { SourceStatusBadge } from "./source-status-badge";
import { StatusSwitch } from "./status-switch";

type SourceRowProps = {
  source: Source;
  onEdit: (source: Source) => void;
  onDelete: (source: Source) => void;
};

export function SourceRow({ source, onEdit, onDelete }: SourceRowProps) {
  const { t } = useTranslation("admin");
  const format = useFormat();
  const toastError = useErrorToast();
  const update = useUpdateSource();

  return (
    <TableRow className={source.enabled ? undefined : "text-muted-foreground"}>
      <TableCell>
        <div className="flex items-center gap-3">
          <SourceKindTag kind={source.kind} />
          <div className="flex min-w-0 flex-col gap-0.5">
            <span className="font-medium text-foreground">{source.name}</span>
            <SourceDetail source={source} />
          </div>
        </div>
      </TableCell>
      <TableCell className="font-mono text-[12.5px]">
        {format.interval(source.interval_minutes)}
      </TableCell>
      <TableCell className="whitespace-normal">
        <SourceStatusBadge source={source} />
      </TableCell>
      <TableCell className="text-[13px] text-subtle-foreground">
        {source.last_run_at ? format.relative(source.last_run_at) : "—"}
      </TableCell>
      <TableCell className="text-right font-mono text-[13px]">
        {format.number(source.items_24h, 0)}
      </TableCell>
      <TableCell>
        <div className="flex items-center justify-end gap-1">
          <CollectSourceButton source={source} />
          <Button
            variant="ghost"
            size="icon"
            className={TOUCH_TARGET}
            onClick={() => onEdit(source)}
            aria-label={t("sources.actions.edit", { name: source.name })}
          >
            <Pencil aria-hidden />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className={`${TOUCH_TARGET} hover:text-negative`}
            onClick={() => onDelete(source)}
            aria-label={t("sources.actions.delete", { name: source.name })}
          >
            <Trash2 aria-hidden />
          </Button>
          <StatusSwitch
            className="ml-2"
            checked={source.enabled}
            disabled={update.isPending}
            onCheckedChange={(enabled) =>
              update.mutate({ id: source.id, body: { enabled } }, { onError: toastError })
            }
            aria-label={t("sources.actions.enabled", { name: source.name })}
          />
        </div>
      </TableCell>
    </TableRow>
  );
}
