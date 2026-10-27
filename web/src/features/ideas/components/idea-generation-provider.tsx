import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";

import { useGenerateIdeas, useInvalidateIdeas } from "../api/ideas-queries";
import { useErrorMessage } from "../hooks/use-error-message";
import {
  IdeaGenerationContext,
  type GenerationOutcome,
  type GenerationRun,
  type IdeaGeneration,
  type ReportBase,
} from "../hooks/use-idea-generation";
import { useIdeaJob } from "../hooks/use-idea-job";
import { useOutcomeSummary } from "../hooks/use-outcome-summary";
import type { GenerateIdeasValues } from "../schemas/idea.schema";
import { GenerateIdeasDialog } from "./generate-ideas-dialog";

/**
 * Follows the user's idea generation on every screen of the app and hosts the
 * "generate ideas" dialog. A run that ends with the dialog closed is announced
 * with a toast.
 */
export function IdeaGenerationProvider({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation("ideas");
  const navigate = useNavigate();
  const errorMessage = useErrorMessage();
  const summary = useOutcomeSummary();
  const invalidate = useInvalidateIdeas();
  const generate = useGenerateIdeas();
  const [run, setRun] = useState<GenerationRun | null>(null);
  const [outcome, setOutcome] = useState<GenerationOutcome | null>(null);
  const [draft, setDraft] = useState<GenerateIdeasValues | null>(null);
  const [dialog, setDialog] = useState<{ open: boolean; report: ReportBase | null }>({
    open: false,
    report: null,
  });
  const [showOutcome, setShowOutcome] = useState(false);

  const { job, running } = useIdeaJob(run?.jobId ?? null, (result) => {
    if (!run) return;
    setRun(null);
    setOutcome({ ...result, run });
    void invalidate();
    if (dialog.open) {
      setShowOutcome(true);
    } else if (result.kind === "success") {
      toast.success(t("dialog.toast", { summary: summary(result) }), {
        action: { label: t("dialog.viewIdeas"), onClick: () => void navigate("/ideas") },
      });
    } else {
      toast.error(errorMessage(result.code));
    }
  });
  const following = run !== null && running;

  const value: IdeaGeneration = {
    run: following ? run : null,
    running: following,
    queued: following && job?.status === "pending",
    isStarting: generate.isPending,
    outcome,
    draft,
    start: (request, base, nextDraft, onError) =>
      generate.mutate(request, {
        onSuccess: (jobId) => {
          setRun({ jobId, count: request.count, base });
          setOutcome(null);
          setShowOutcome(false);
          setDraft(nextDraft);
        },
        onError,
      }),
    dialog: {
      ...dialog,
      view: following ? "progress" : showOutcome && outcome ? "result" : "form",
    },
    openDialog: (report) => {
      setShowOutcome(false);
      setDialog({ open: true, report: report ?? null });
    },
    closeDialog: () => {
      setShowOutcome(false);
      setDialog((current) => ({ ...current, open: false }));
    },
    restart: () => setShowOutcome(false),
  };

  return (
    <IdeaGenerationContext value={value}>
      {children}
      <GenerateIdeasDialog />
    </IdeaGenerationContext>
  );
}
