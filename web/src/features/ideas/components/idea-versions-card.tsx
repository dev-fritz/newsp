import { Link } from "react-router";
import { useTranslation } from "react-i18next";

import { useIdeaFormat } from "../hooks/use-idea-format";
import type { Idea } from "../schemas/idea-response.schema";
import { IdeaBadge } from "./idea-badge";
import { VersionOriginIcon } from "./version-origin-icon";

/** Version timeline, newest first, with links to compare each with the one before. */
export function IdeaVersionsCard({ idea }: { idea: Idea }) {
  const { t } = useTranslation("ideas");
  const format = useIdeaFormat();
  const versions = idea.versions.toSorted((a, b) => b.version - a.version);
  const compare = (from: number, to: number) => `/ideas/${idea.id}/compare?from=${from}&to=${to}`;

  return (
    <section
      aria-labelledby="versions-title"
      className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5"
    >
      <div className="flex items-baseline justify-between gap-3">
        <h2 id="versions-title" className="kicker text-foreground">
          {t("versions.title")}
        </h2>
        {versions.length > 1 && (
          <Link
            to={compare(idea.current_version - 1, idea.current_version)}
            className="text-[13px] text-signal underline-offset-2 hover:underline"
          >
            {t("versions.compare")}
          </Link>
        )}
      </div>
      <ol className="m-0 flex list-none flex-col p-0">
        {versions.map((version, index) => {
          const isCurrent = version.version === idea.current_version;
          const isLast = index === versions.length - 1;
          return (
            <li
              key={version.version}
              className={`grid grid-cols-[28px_minmax(0,1fr)] gap-3 ${isLast ? "" : "pb-4"}`}
            >
              <VersionOriginIcon origin={version.origin} current={isCurrent} />
              <span className="flex min-w-0 flex-col gap-[3px]">
                <span className="flex flex-wrap items-baseline gap-2">
                  <strong className="text-sm font-semibold">
                    {t("versions.label", {
                      version: version.version,
                      origin: t(`origin.${version.origin}`),
                    })}
                  </strong>
                  {isCurrent && (
                    <IdeaBadge tone="signal" className="h-[18px] text-[10.5px]">
                      {t("versions.current")}
                    </IdeaBadge>
                  )}
                </span>
                <span className="font-mono text-[11.5px] text-subtle-foreground">
                  {format.dateTime(version.created_at)}
                </span>
                {version.change_note && (
                  <span className="text-[13px] break-words text-muted-foreground">
                    {version.change_note}
                  </span>
                )}
                {version.version > 1 && (
                  <Link
                    to={compare(version.version - 1, version.version)}
                    className="w-fit text-[13px] text-signal underline-offset-2 hover:underline"
                  >
                    {t("versions.seeChanges", { version: version.version })}
                  </Link>
                )}
              </span>
            </li>
          );
        })}
      </ol>
    </section>
  );
}
