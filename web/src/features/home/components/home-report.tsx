import { ArrowRight } from "lucide-react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";

import { EmptyState, ErrorState } from "@/components/common/states";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ReportMarkdown, useLatestReport } from "@/features/reports";

import { reportExcerpt } from "../lib/excerpt";

const EXCERPT_SECTIONS = 2;

function ReportSkeleton() {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-4" aria-busy="true" aria-label={t("states.loading")}>
      <Skeleton className="h-5 w-48" />
      <Skeleton className="h-12 w-full" />
      <Skeleton className="h-20 w-full" />
      <Skeleton className="h-40 w-full rounded-lg" />
    </div>
  );
}

/** The lead story: the latest finished report (title, summary, callout and first sections). */
export function HomeReport() {
  const { t, i18n } = useTranslation("home");
  const query = useLatestReport();

  if (query.isPending) return <ReportSkeleton />;
  if (query.isError) return <ErrorState error={query.error} onRetry={() => void query.refetch()} />;

  const report = query.data;
  if (!report?.id) {
    return (
      <EmptyState
        title={t("report.empty.title")}
        body={t("report.empty.body")}
        action={
          <Button asChild className="h-10 pointer-coarse:h-11">
            <Link to="/reports">{t("report.empty.action")}</Link>
          </Button>
        }
      />
    );
  }

  const window =
    report.window_start && report.window_end
      ? new Intl.DateTimeFormat(i18n.language, {
          day: "numeric",
          month: "short",
          hour: "2-digit",
          minute: "2-digit",
        })
      : null;

  return (
    <article className="flex min-w-0 flex-col gap-5">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
        <Badge className="border-transparent bg-signal-soft text-signal-ink">
          {t("report.badge")}
        </Badge>
        {window && report.window_start && report.window_end && (
          <span className="kicker">
            {window.format(new Date(report.window_start))} →{" "}
            {window.format(new Date(report.window_end))}
          </span>
        )}
      </div>
      <h2 className="max-w-[32ch] font-serif text-[32px] leading-[1.1] font-medium text-balance sm:text-[40px]">
        <Link
          to={`/reports/${report.id}`}
          className="rounded-sm text-foreground no-underline outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {report.title}
        </Link>
      </h2>
      {report.summary && (
        <p className="max-w-3xl font-serif text-[19px] leading-[1.55] text-muted-foreground italic">
          {report.summary}
        </p>
      )}
      {report.content_md && (
        <ReportMarkdown
          content={reportExcerpt(report.content_md, EXCERPT_SECTIONS)}
          citations={report.citations}
        />
      )}
      <Button asChild variant="outline" className="h-10 w-fit pointer-coarse:h-11">
        <Link to={`/reports/${report.id}`}>
          {t("report.readFull")}
          <ArrowRight aria-hidden />
        </Link>
      </Button>
    </article>
  );
}
