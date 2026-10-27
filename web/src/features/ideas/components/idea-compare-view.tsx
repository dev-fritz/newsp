import { ArrowRight, ChevronDown } from "lucide-react";
import { useMemo, useState } from "react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";

import { EmptyState, ErrorState } from "@/components/common/states";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

import { useIdea, useIdeaVersion } from "../api/ideas-queries";
import { diffVersions } from "../lib/idea-diff";
import { resolveCompareVersions } from "../lib/compare-versions";
import { CompareSummary } from "./compare-summary";
import { DiffLegend } from "./diff-legend";
import { DiffSection } from "./diff-section";
import { Segmented } from "./segmented";
import { VersionSelect } from "./version-select";

type View = "changes" | "all";

type Props = {
  id: string;
  /** Requested versions from the URL; unknown ones fall back to previous → current. */
  from?: number;
  to?: number;
  onVersionsChange: (versions: { from: number; to: number }) => void;
};

/** Side-by-side history of an idea: pick two versions and see what changed, section by section. */
export function IdeaCompareView({ id, from, to, onVersionsChange }: Props) {
  const { t } = useTranslation(["ideas", "common"]);
  const idea = useIdea(id);
  const [view, setView] = useState<View>("changes");

  const versions = idea.data?.versions ?? [];
  const picked = resolveCompareVersions(
    versions.map((v) => v.version),
    idea.data?.current_version ?? 0,
    from,
    to,
  );
  const fromVersion = useIdeaVersion(id, idea.data ? picked.from : undefined);
  const toVersion = useIdeaVersion(id, idea.data ? picked.to : undefined);
  const fromData = fromVersion.data;
  const toData = toVersion.data;
  const diff = useMemo(
    () => (fromData && toData ? diffVersions(fromData, toData) : null),
    [fromData, toData],
  );

  if (idea.isPending) return <Skeleton className="h-96 w-full rounded-xl" aria-busy="true" />;
  if (idea.isError) {
    return idea.error.status === 404 ? (
      <EmptyState
        title={t("ideas:detail.notFoundTitle")}
        body={t("ideas:detail.notFoundBody")}
        action={
          <Button asChild variant="outline">
            <Link to="/ideas">{t("ideas:detail.backToList")}</Link>
          </Button>
        }
      />
    ) : (
      <ErrorState error={idea.error} onRetry={() => void idea.refetch()} />
    );
  }

  const data = idea.data;
  const back = (
    <Button asChild variant="secondary">
      <Link to={`/ideas/${data.id}`}>{t("ideas:compare.back")}</Link>
    </Button>
  );

  if (data.versions.length < 2) {
    return (
      <EmptyState
        title={t("ideas:compare.singleTitle")}
        body={t("ideas:compare.singleBody")}
        action={back}
      />
    );
  }

  const onlyChanges = view === "changes";
  const sections = diff
    ? onlyChanges
      ? diff.sections.filter((s) => s.changed)
      : diff.sections
    : [];
  const hidden = diff ? diff.sections.filter((s) => !s.changed) : [];
  const failed = fromVersion.isError ? fromVersion : toVersion.isError ? toVersion : null;

  return (
    <div className="mx-auto flex w-full max-w-[900px] flex-col gap-[26px]">
      <nav
        aria-label={t("ideas:detail.breadcrumb")}
        className="flex flex-wrap items-center gap-2 text-[13.5px]"
      >
        <Link to="/ideas" className="text-muted-foreground hover:text-foreground">
          {t("ideas:list.title")}
        </Link>
        <span className="text-subtle-foreground" aria-hidden>
          /
        </span>
        <Link to={`/ideas/${data.id}`} className="text-muted-foreground hover:text-foreground">
          {data.current.title}
        </Link>
        <span className="text-subtle-foreground" aria-hidden>
          /
        </span>
        <span aria-current="page">{t("ideas:compare.crumb")}</span>
      </nav>

      <header className="flex flex-col gap-[18px]">
        <h1 className="font-serif text-4xl leading-[1.08] font-medium sm:text-[40px]">
          {t("ideas:compare.title")}
        </h1>
        <div className="flex flex-wrap items-end gap-3">
          <VersionSelect
            id="compare-from"
            label={t("ideas:compare.from")}
            value={picked.from}
            versions={data.versions}
            onChange={(next) => onVersionsChange({ from: next, to: picked.to })}
          />
          <ArrowRight className="mb-3 hidden size-4 text-subtle-foreground sm:block" aria-hidden />
          <VersionSelect
            id="compare-to"
            label={t("ideas:compare.to")}
            value={picked.to}
            versions={data.versions}
            onChange={(next) => onVersionsChange({ from: picked.from, to: next })}
          />
          <Segmented
            label={t("ideas:compare.view")}
            value={view}
            onChange={setView}
            options={[
              { value: "changes", label: t("ideas:compare.onlyChanges") },
              { value: "all", label: t("ideas:compare.whole") },
            ]}
            className="sm:ml-auto"
          />
        </div>
      </header>

      {failed ? (
        <ErrorState error={failed.error} onRetry={() => void failed.refetch()} />
      ) : !diff || !toVersion.data ? (
        <div
          className="flex flex-col gap-4"
          aria-busy="true"
          aria-label={t("common:states.loading")}
        >
          <Skeleton className="h-40 w-full rounded-xl" />
          <Skeleton className="h-32 w-full rounded-xl" />
        </div>
      ) : (
        <>
          <CompareSummary target={toVersion.data} diff={diff} />
          <DiffLegend />
          {sections.length === 0 && (
            <p className="text-muted-foreground">{t("ideas:compare.noChanges")}</p>
          )}
          {sections.map((section) => (
            <DiffSection key={section.id} section={section} onlyChanges={onlyChanges} />
          ))}
          {onlyChanges && hidden.length > 0 && (
            <Button variant="ghost" className="self-start" onClick={() => setView("all")}>
              <ChevronDown aria-hidden />
              {t("ideas:compare.showUnchanged", {
                count: hidden.length,
                names: hidden.map((s) => t(`ideas:compare.sections.${s.id}`)).join(", "),
              })}
            </Button>
          )}
        </>
      )}

      <div className="flex flex-wrap justify-between gap-2.5 pt-2">{back}</div>
    </div>
  );
}
