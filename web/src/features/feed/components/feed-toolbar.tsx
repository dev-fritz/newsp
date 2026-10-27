import { Search } from "lucide-react";
import { useEffect, useEffectEvent, useState } from "react";
import { useTranslation } from "react-i18next";

import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

import { useFeedSources } from "../api/feed-queries";
import {
  FEED_PERIODS,
  FEED_SORTS,
  MAX_QUERY_LENGTH,
  type FeedFilters,
} from "../schemas/feed-filters.schema";
import { Segmented } from "./segmented";

const ALL_SOURCES = "all";
const DEBOUNCE_MS = 300;

type Props = {
  filters: FeedFilters;
  onChange: (patch: Partial<FeedFilters>) => void;
};

/** Search, source, period and sort controls; every value lives in the URL via `onChange`. */
export function FeedToolbar({ filters, onChange }: Props) {
  const { t } = useTranslation("feed");
  const sources = useFeedSources();
  const [text, setText] = useState(filters.q);

  // The URL changed from outside (e.g. "clear filters"): show it in the box.
  const [seenQ, setSeenQ] = useState(filters.q);
  if (filters.q !== seenQ) {
    setSeenQ(filters.q);
    if (filters.q !== text.trim()) setText(filters.q);
  }

  const commit = useEffectEvent((value: string) => {
    if (value !== filters.q) onChange({ q: value });
  });
  useEffect(() => {
    const timer = setTimeout(() => commit(text.trim()), DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [text]);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative flex w-full items-center sm:w-80">
          <Search
            className="pointer-events-none absolute left-3 size-4 text-subtle-foreground"
            aria-hidden
          />
          <Input
            type="search"
            value={text}
            onChange={(event) => setText(event.target.value)}
            placeholder={t("filters.searchPlaceholder")}
            aria-label={t("filters.search")}
            maxLength={MAX_QUERY_LENGTH}
            className="h-10 pl-9"
          />
        </div>
        <Select
          value={filters.source ?? ALL_SOURCES}
          onValueChange={(value) => onChange({ source: value === ALL_SOURCES ? undefined : value })}
        >
          <SelectTrigger
            aria-label={t("filters.source")}
            className="h-10 w-full sm:w-56 pointer-coarse:h-11"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_SOURCES}>{t("filters.allSources")}</SelectItem>
            {sources.data?.map((source) =>
              source.id ? (
                <SelectItem key={source.id} value={source.id}>
                  {source.name}
                </SelectItem>
              ) : null,
            )}
          </SelectContent>
        </Select>
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <Segmented
          label={t("filters.period")}
          value={filters.period}
          onChange={(period) => onChange({ period })}
          options={FEED_PERIODS.map((value) => ({ value, label: t(`filters.periods.${value}`) }))}
          className="overflow-x-auto"
        />
        <Segmented
          label={t("filters.sort")}
          value={filters.sort}
          onChange={(sort) => onChange({ sort })}
          options={FEED_SORTS.map((value) => ({ value, label: t(`filters.sorts.${value}`) }))}
        />
      </div>
    </div>
  );
}
