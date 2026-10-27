import { Dialog, DialogContent } from "@/components/ui/dialog";

import { useIdeaGeneration } from "../hooks/use-idea-generation";
import { GenerateIdeasForm } from "./generate-ideas-form";
import { GenerationProgressView } from "./generation-progress-view";
import { GenerationResultView } from "./generation-result-view";

/** "Gerar ideias": the form, then the run's progress and its result, in one dialog. */
export function GenerateIdeasDialog() {
  const { dialog, closeDialog } = useIdeaGeneration();
  return (
    <Dialog open={dialog.open} onOpenChange={(open) => !open && closeDialog()}>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto p-6 sm:max-w-[560px]">
        {dialog.view === "form" && <GenerateIdeasForm report={dialog.report} />}
        {dialog.view === "progress" && <GenerationProgressView />}
        {dialog.view === "result" && <GenerationResultView />}
      </DialogContent>
    </Dialog>
  );
}
