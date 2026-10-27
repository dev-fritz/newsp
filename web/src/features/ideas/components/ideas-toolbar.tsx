import { Search } from "lucide-react";
import { useEffect, useEffectEvent, useState } from "react";
import { useTranslation } from "react-i18next";

import { Input } from "@/components/ui/input";

import { useDebouncedValue } from "../hooks/use-debounced-value";
import { IDEA_STATUS_FILTERS, type IdeaStatusFilter } from "../schemas/idea.schema";
import { Segmented } from "./segmented";

type Props = {
  status: IdeaStatusFilter;
  query: string;
  onStatusChange: (status: IdeaStatusFilter) => void;
  onQueryChange: (query: string) => void;
};

/** Status filter and debounced search; both live in the URL via the callbacks. */
export function IdeasToolbar({ status, query, onStatusChange, onQueryChange }: Props) {
  const { t } = useTranslation("ideas");
  const [text, setText] = useState(query);
  const debounced = useDebouncedValue(text.trim(), 300);
  const commit = useEffectEvent((value: string) => {
    if (value !== query) onQueryChange(value);
  });
  useEffect(() => commit(debounced), [debounced]);

  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <Segmented
        label={t("filters.label")}
        value={status}
        onChange={onStatusChange}
        options={IDEA_STATUS_FILTERS.map((value) => ({
          value,
          label: t(`filters.${value}`),
        }))}
        className="overflow-x-auto"
      />
      <div className="relative flex w-full items-center sm:w-72">
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
          maxLength={100}
          className="h-10 pl-9"
        />
      </div>
    </div>
  );
}
