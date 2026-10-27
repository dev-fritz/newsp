import { ArrowRight, Check, CircleAlert } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { Button } from "@/components/ui/button";
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

import { useIdeasByIds } from "../api/ideas-queries";
import { useErrorMessage } from "../hooks/use-error-message";
import { useIdeaGeneration } from "../hooks/use-idea-generation";
import { useOutcomeSummary } from "../hooks/use-outcome-summary";
import { IdeaBadge } from "./idea-badge";

/** The ideas a run created or updated, each linking to its page. */
function OutcomeIdeas({ created, merged }: { created: string[]; merged: string[] }) {
  const { t } = useTranslation("ideas");
  const { closeDialog } = useIdeaGeneration();
  const ids = [...created, ...merged];
  const ideas = useIdeasByIds(ids);
  if (ids.length === 0) return null;
  return (
    <ul className="m-0 flex list-none flex-col overflow-hidden rounded-lg border border-border p-0">
      {ids.map((id, i) => {
        const idea = ideas[i]?.data;
        const isNew = i < created.length;
        return (
          <li key={id} className="border-b border-border last:border-b-0">
            <Link
              to={`/ideas/${encodeURIComponent(id)}`}
              onClick={closeDialog}
              className="flex min-h-11 items-center gap-3 px-4 py-3.5 text-foreground outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:ring-inset"
            >
              {isNew ? (
                <IdeaBadge tone="marker">{t("dialog.newBadge")}</IdeaBadge>
              ) : (
                <IdeaBadge tone="signal">
                  {idea
                    ? t("dialog.updatedBadge", { version: idea.current_version })
                    : t("dialog.updatedBadgeShort")}
                </IdeaBadge>
              )}
              <span className="min-w-0 flex-1 font-serif text-[17px] leading-snug">
                {idea ? (
                  idea.current.title
                ) : (
                  <span aria-hidden className="block h-4 w-3/4 animate-pulse rounded-md bg-muted" />
                )}
              </span>
              <ArrowRight aria-hidden className="size-4 shrink-0 text-subtle-foreground" />
            </Link>
          </li>
        );
      })}
    </ul>
  );
}

/** How the last run ended: the ideas it produced, or why it failed. */
export function GenerationResultView() {
  const { t } = useTranslation(["ideas", "common"]);
  const errorMessage = useErrorMessage();
  const summary = useOutcomeSummary();
  const { outcome, closeDialog, restart } = useIdeaGeneration();
  if (!outcome) return null;

  if (outcome.kind === "failure") {
    return (
      <div className="flex flex-col gap-5">
        <DialogHeader className="pr-8">
          <span
            role="alert"
            className="inline-flex items-center gap-2 text-[13px] font-medium text-negative"
          >
            <CircleAlert aria-hidden className="size-4" />
            {t("ideas:dialog.failed")}
          </span>
          <DialogTitle className="font-serif text-[26px] leading-tight font-medium">
            {t("ideas:dialog.failedTitle")}
          </DialogTitle>
          <DialogDescription>{errorMessage(outcome.code)}</DialogDescription>
        </DialogHeader>
        <DialogFooter className="border-t border-border pt-4">
          <Button
            type="button"
            variant="ghost"
            className="h-9 pointer-coarse:h-11"
            onClick={closeDialog}
          >
            {t("common:actions.close")}
          </Button>
          <Button type="button" className="h-9 pointer-coarse:h-11" onClick={restart}>
            {t("ideas:dialog.retry")}
          </Button>
        </DialogFooter>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-5">
      <DialogHeader className="pr-8">
        <output className="inline-flex items-center gap-2 text-[13px] font-medium text-positive">
          <Check aria-hidden className="size-4" />
          {t("ideas:dialog.done")}
        </output>
        <DialogTitle className="font-serif text-[26px] leading-tight font-medium">
          {summary(outcome)}
        </DialogTitle>
        <DialogDescription>{t("ideas:dialog.resultHint")}</DialogDescription>
      </DialogHeader>
      <OutcomeIdeas created={outcome.created} merged={outcome.merged} />
      <DialogFooter className="border-t border-border pt-4">
        <Button type="button" variant="ghost" className="h-9 pointer-coarse:h-11" onClick={restart}>
          {t("ideas:dialog.again")}
        </Button>
        <Button asChild className="h-9 pointer-coarse:h-11">
          <Link to="/ideas" onClick={closeDialog}>
            {t("ideas:dialog.viewIdeas")}
          </Link>
        </Button>
      </DialogFooter>
    </div>
  );
}
