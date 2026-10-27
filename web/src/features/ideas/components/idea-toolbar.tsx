import { Link } from "react-router";
import { useTranslation } from "react-i18next";

import type { Idea } from "../schemas/idea-response.schema";
import { IdeaActions } from "./idea-actions";
import { IdeaStatusControl } from "./idea-status-control";

type Props = {
  idea: Idea;
  onEdit: () => void;
  onDeepen: () => void;
  deepening: boolean;
};

/** Breadcrumb, triage switch and (on desktop) the edit and deepen actions. */
export function IdeaToolbar({ idea, onEdit, onDeepen, deepening }: Props) {
  const { t } = useTranslation("ideas");
  return (
    <div className="flex flex-wrap items-center justify-between gap-3.5">
      <nav
        aria-label={t("detail.breadcrumb")}
        className="flex min-w-0 items-center gap-2 text-[13.5px]"
      >
        <Link to="/ideas" className="text-muted-foreground hover:text-foreground">
          {t("list.title")}
        </Link>
        <span className="text-subtle-foreground" aria-hidden>
          /
        </span>
        <span className="truncate" aria-current="page">
          {idea.current.title}
        </span>
      </nav>
      <div className="flex w-full flex-wrap items-center gap-2 lg:w-auto">
        <IdeaStatusControl
          ideaId={idea.id}
          status={idea.status}
          className="grid w-full auto-cols-fr grid-flow-col lg:inline-flex lg:w-auto"
        />
        <div className="hidden items-center gap-2 lg:flex">
          <IdeaActions onEdit={onEdit} onDeepen={onDeepen} deepening={deepening} />
        </div>
      </div>
    </div>
  );
}
