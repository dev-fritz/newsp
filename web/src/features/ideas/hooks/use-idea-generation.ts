import { createContext, useContext } from "react";

import type { ApiError } from "@/lib/api-error";

import type { GenerateIdeasRequest } from "../api/ideas-api";
import type { GenerateIdeasValues } from "../schemas/idea.schema";
import type { JobOutcome } from "./use-idea-job";

/** A finished report to generate from, as the reports feature describes it. */
export type ReportBase = { id: string; title: string; meta: string };

/** What a generation starts from, shown while it runs. */
export type GenerationBase =
  | {
      kind: "window";
      windowDays: GenerateIdeasValues["window_days"];
      focus: string;
      items: number;
    }
  | { kind: "report"; report: ReportBase };

export type GenerationRun = { jobId: string; count: number; base: GenerationBase };
export type GenerationOutcome = JobOutcome & { run: GenerationRun };
export type GenerationView = "form" | "progress" | "result";

export type IdeaGeneration = {
  /** The job being followed until it reaches a final state. */
  run: GenerationRun | null;
  running: boolean;
  queued: boolean;
  isStarting: boolean;
  /** The last finished run, for the dialog's result view. */
  outcome: GenerationOutcome | null;
  /** Form values of the last run started from the form, so "generate more" keeps them. */
  draft: GenerateIdeasValues | null;
  start: (
    request: GenerateIdeasRequest,
    base: GenerationBase,
    draft: GenerateIdeasValues | null,
    onError: (error: ApiError) => void,
  ) => void;
  dialog: { open: boolean; report: ReportBase | null; view: GenerationView };
  /** Opens the dialog, from a report when one is given; a running job shows its progress. */
  openDialog: (report?: ReportBase) => void;
  closeDialog: () => void;
  /** Leaves the result view for the form. */
  restart: () => void;
};

export const IdeaGenerationContext = createContext<IdeaGeneration | null>(null);

/** The user's idea generation, followed across screens by `IdeaGenerationProvider`. */
export function useIdeaGeneration(): IdeaGeneration {
  const generation = useContext(IdeaGenerationContext);
  if (!generation) throw new Error("useIdeaGeneration needs an IdeaGenerationProvider");
  return generation;
}
