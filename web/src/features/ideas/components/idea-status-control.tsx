import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { useUpdateIdeaStatus } from "../api/ideas-queries";
import { useErrorMessage } from "../hooks/use-error-message";
import type { IdeaStatus } from "../schemas/idea-response.schema";
import { Segmented } from "./segmented";

const STATUSES = ["new", "saved", "discarded"] as const satisfies readonly IdeaStatus[];

type Props = { ideaId: string; status: IdeaStatus; className?: string };

/** Triage switch (new / saved / discarded); the change shows at once and rolls back on failure. */
export function IdeaStatusControl({ ideaId, status, className }: Props) {
  const { t } = useTranslation("ideas");
  const errorMessage = useErrorMessage();
  const update = useUpdateIdeaStatus(ideaId);
  return (
    <Segmented
      label={t("detail.statusLabel")}
      value={status}
      options={STATUSES.map((value) => ({ value, label: t(`status.${value}`) }))}
      onChange={(next) => {
        if (next === status) return;
        update.mutate(next, { onError: (error) => toast.error(errorMessage(error.code)) });
      }}
      className={className}
    />
  );
}
