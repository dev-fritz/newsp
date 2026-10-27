import { useTranslation } from "react-i18next";

import { PageHeader } from "@/components/common/page-header";

import { useFeedFilters } from "../hooks/use-feed-filters";
import { FeedList } from "./feed-list";
import { FeedToolbar } from "./feed-toolbar";

/** The `/feed` screen: filters in the URL and the paginated item list. */
export function FeedScreen() {
  const { t } = useTranslation("feed");
  const { filters, hasFilters, setFilters, clearFilters } = useFeedFilters();
  return (
    <div className="flex min-w-0 flex-col gap-6">
      <PageHeader kicker={t("kicker")} title={t("title")} />
      <FeedToolbar filters={filters} onChange={setFilters} />
      <FeedList filters={filters} hasFilters={hasFilters} onClear={clearFilters} />
    </div>
  );
}
