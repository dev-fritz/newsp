import { Link } from "react-router";
import { useTranslation } from "react-i18next";

import { EmptyState, ErrorState } from "@/components/common/states";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useCan } from "@/features/auth";
import { ItemLink, SourceTag, useItemFormat, useTopItems } from "@/features/feed";

/** "Trending now": the most engaging items of the last 24 hours. */
export function HomeTrending() {
  const { t } = useTranslation(["home", "common"]);
  const query = useTopItems();
  const format = useItemFormat();
  const canManageSources = useCan("sources:manage");

  let body: React.ReactNode;
  if (query.isPending) {
    body = (
      <div className="flex flex-col gap-3" aria-busy="true" aria-label={t("common:states.loading")}>
        {Array.from({ length: 5 }, (_, i) => (
          <Skeleton key={i} className="h-14 w-full" />
        ))}
      </div>
    );
  } else if (query.isError) {
    body = <ErrorState error={query.error} onRetry={() => void query.refetch()} />;
  } else if (query.data.length === 0) {
    body = canManageSources ? (
      <EmptyState
        title={t("home:trending.empty.title")}
        body={t("home:trending.empty.adminBody")}
        action={
          <Button asChild className="h-10 pointer-coarse:h-11">
            <Link to="/admin/sources">{t("home:trending.empty.adminAction")}</Link>
          </Button>
        }
      />
    ) : (
      <EmptyState
        title={t("home:trending.empty.title")}
        body={t("home:trending.empty.memberBody")}
      />
    );
  } else {
    body = (
      <ol className="m-0 flex list-none flex-col p-0">
        {query.data.map((item, index) => {
          const engagement = format.engagement(item);
          return (
            <li
              key={item.id ?? index}
              className="flex gap-3 border-b border-border py-3 first:pt-0 last:border-b-0"
            >
              <span
                aria-hidden
                className="w-5 shrink-0 pt-0.5 font-serif text-lg leading-tight text-subtle-foreground"
              >
                {index + 1}
              </span>
              <div className="flex min-w-0 flex-col gap-1">
                <ItemLink
                  url={item.url}
                  title={item.title?.trim() || t("home:trending.untitled")}
                  className="font-serif text-[17px] leading-[1.3] [overflow-wrap:anywhere]"
                />
                <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-subtle-foreground">
                  <SourceTag kind={item.source?.kind} name={item.source?.name} />
                  {engagement && <span className="font-mono">{engagement}</span>}
                  <time dateTime={item.published_at} className="font-mono">
                    {format.relative(item.published_at)}
                  </time>
                </span>
              </div>
            </li>
          );
        })}
      </ol>
    );
  }

  return (
    <section aria-labelledby="home-trending" className="flex min-w-0 flex-col gap-4">
      <div className="flex items-baseline justify-between gap-3">
        <h2 id="home-trending" className="font-serif text-2xl font-medium">
          {t("home:trending.title")}
        </h2>
        <Link
          to="/feed?sort=engagement&period=24h"
          className="rounded-sm text-sm text-signal outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {t("home:trending.seeAll")}
        </Link>
      </div>
      {body}
    </section>
  );
}
