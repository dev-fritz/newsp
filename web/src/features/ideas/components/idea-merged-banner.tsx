import { GitMerge } from "lucide-react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";

type Props = { ideaId: string; version: number; newEvidence: number };

/** Tells the reader that a similar idea was merged into this one, with a link to what changed. */
export function IdeaMergedBanner({ ideaId, version, newEvidence }: Props) {
  const { t } = useTranslation("ideas");
  return (
    <Link
      to={`/ideas/${ideaId}/compare?from=${version - 1}&to=${version}`}
      className="flex items-start gap-3.5 rounded-xl bg-signal-soft px-4 py-3.5 text-signal-ink no-underline outline-none focus-visible:ring-3 focus-visible:ring-ring/50 sm:items-center sm:px-[18px]"
    >
      <GitMerge className="mt-0.5 size-5 shrink-0 sm:mt-0" aria-hidden />
      <span className="flex-1 text-sm leading-[1.45]">
        <strong className="font-semibold">{t("banner.title", { version })}</strong>{" "}
        {t("banner.body")}
        {newEvidence > 0 && <> {t("banner.evidence", { count: newEvidence })}</>}
        <span className="font-medium sm:hidden"> {t("banner.action")}</span>
      </span>
      <span className="hidden text-[13.5px] font-medium whitespace-nowrap sm:inline">
        {t("banner.action")}
      </span>
    </Link>
  );
}
