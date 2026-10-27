import { Check, Search, X } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import {
  SourceTag,
  useItemFormat,
  useItemSearch,
  type FeedItem,
  type FeedPeriod,
} from "@/features/feed";

import { useDebouncedValue } from "../hooks/use-debounced-value";

export type PickedItem = { id: string; title: string; sourceKind?: string; sourceName?: string };

type Props = {
  value: PickedItem[];
  onChange: (next: PickedItem[]) => void;
  windowDays: 1 | 7 | 30;
  max?: number;
  labelledBy?: string;
  error?: string;
};

const PERIOD: Record<Props["windowDays"], FeedPeriod> = { 1: "24h", 7: "7d", 30: "30d" };

/** Search the collected feed and pick up to `max` items to seed an idea generation. */
export function ItemPicker({ value, onChange, windowDays, max = 5, labelledBy, error }: Props) {
  const { t } = useTranslation("ideas");
  const format = useItemFormat();
  const [text, setText] = useState("");
  const query = useDebouncedValue(text.trim(), 300);
  const search = useItemSearch(query, PERIOD[windowDays]);
  const atLimit = value.length >= max;
  const items = search.data ?? [];
  const refetching = search.isPlaceholderData || (search.isFetching && !search.isPending);

  function toggle(item: FeedItem) {
    if (!item.id) return;
    const id = item.id;
    if (value.some((v) => v.id === id)) {
      onChange(value.filter((v) => v.id !== id));
      return;
    }
    if (atLimit) return;
    onChange([
      ...value,
      {
        id,
        title: item.title?.trim() || t("picker.untitled"),
        sourceKind: item.source?.kind,
        sourceName: item.source?.name,
      },
    ]);
  }

  return (
    <fieldset
      aria-labelledby={labelledBy}
      className="m-0 flex min-w-0 flex-col gap-2.5 border-0 p-0"
    >
      <div className="flex flex-col items-end gap-1">
        <span className="font-mono text-[11.5px] text-subtle-foreground">
          {t("picker.count", { count: value.length, max })}
        </span>
        <p className="w-full text-xs text-subtle-foreground">{t("picker.hint")}</p>
      </div>

      {value.length > 0 && (
        <ul aria-label={t("picker.selectedLabel")} className="flex flex-wrap gap-1.5">
          {value.map((p) => (
            <li
              key={p.id}
              className="inline-flex max-w-full items-center gap-1.5 rounded-full border border-border bg-card py-1 pr-1 pl-2.5 text-[12.5px]"
            >
              <SourceTag kind={p.sourceKind} name={p.sourceName} />
              <span className="max-w-[260px] truncate">{p.title}</span>
              <button
                type="button"
                aria-label={t("picker.remove", { title: p.title })}
                onClick={() => onChange(value.filter((v) => v.id !== p.id))}
                className="inline-flex size-5 shrink-0 items-center justify-center rounded-full text-muted-foreground outline-none hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 pointer-coarse:size-8"
              >
                <X className="size-3.5" aria-hidden />
              </button>
            </li>
          ))}
        </ul>
      )}

      <div className="relative">
        <Search
          className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-subtle-foreground"
          aria-hidden
        />
        <input
          type="search"
          value={text}
          maxLength={200}
          onChange={(e) => setText(e.target.value)}
          placeholder={t("picker.search")}
          aria-label={t("picker.search")}
          className="h-10 w-full rounded-lg border border-border bg-card pr-3 pl-9 text-[13.5px] outline-none placeholder:text-subtle-foreground focus-visible:ring-3 focus-visible:ring-ring/50 pointer-coarse:h-11 pointer-coarse:text-base"
        />
      </div>

      <div className="rounded-lg border border-border bg-card">
        <p className="kicker border-b border-border px-3 py-2">
          {query ? t("picker.results") : t("picker.trending")}
        </p>
        {search.isPending ? (
          <div className="flex flex-col gap-3 p-3" aria-hidden>
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : search.isError && !search.data ? (
          <div className="flex items-center justify-between gap-3 p-3 text-[13px] text-muted-foreground">
            <span role="alert">{t("picker.error")}</span>
            <button
              type="button"
              onClick={() => void search.refetch()}
              className="rounded-md border border-border px-2.5 py-1 text-[13px] text-foreground outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50 pointer-coarse:min-h-11"
            >
              {t("picker.retry")}
            </button>
          </div>
        ) : items.length === 0 ? (
          <p className="p-3 text-[13px] text-muted-foreground">{t("picker.empty")}</p>
        ) : (
          <ul
            aria-busy={refetching}
            className={cn("flex flex-col transition-opacity", refetching && "opacity-60")}
          >
            {items.map((item, i) => {
              const selected = value.some((v) => v.id === item.id);
              const meta = [format.engagement(item), format.relative(item.published_at)]
                .filter(Boolean)
                .join(" · ");
              return (
                <li key={item.id ?? i} className="border-b border-border last:border-b-0">
                  <button
                    type="button"
                    aria-pressed={selected}
                    disabled={!item.id || (!selected && atLimit)}
                    onClick={() => toggle(item)}
                    className={cn(
                      "flex min-h-11 w-full items-start gap-3 px-3 py-2.5 text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:ring-inset disabled:opacity-50",
                      selected ? "bg-signal-soft" : "hover:bg-muted",
                    )}
                  >
                    <span className="flex min-w-0 flex-1 flex-col gap-1">
                      <span className="flex items-center gap-2">
                        <SourceTag kind={item.source?.kind} name={item.source?.name} />
                      </span>
                      <span className="line-clamp-2 text-[13.5px] leading-snug [overflow-wrap:anywhere]">
                        {item.title?.trim() || t("picker.untitled")}
                      </span>
                      {meta && (
                        <span className="font-mono text-[11px] text-subtle-foreground">{meta}</span>
                      )}
                    </span>
                    <span
                      aria-hidden
                      className={cn(
                        "mt-0.5 flex size-[18px] shrink-0 items-center justify-center rounded-[5px] border",
                        selected
                          ? "border-primary bg-primary text-primary-foreground"
                          : "border-border-strong",
                      )}
                    >
                      {selected && <Check className="size-3" />}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
        {atLimit && (
          <p className="border-t border-border px-3 py-2 text-xs text-subtle-foreground">
            {t("picker.limit", { max })}
          </p>
        )}
      </div>

      {error && (
        <p role="alert" className="text-xs text-negative">
          {error}
        </p>
      )}
    </fieldset>
  );
}
