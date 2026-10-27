import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import type { Evidence } from "../schemas/idea-response.schema";
import { SourceTag } from "./source-tag";

type Entry = { item: Evidence; sign: "+" | "−" | "" };

function EvidenceRow({ item, sign }: Entry) {
  const { t } = useTranslation("ideas");
  return (
    <li
      className={cn(
        "flex items-baseline gap-3 rounded-lg px-3 py-2.5",
        sign === "+" && "bg-positive-soft",
        sign === "−" && "bg-negative-soft",
      )}
    >
      <span
        className={cn(
          "w-3 shrink-0 text-center font-mono text-xs font-semibold",
          sign === "+" && "text-positive",
          sign === "−" && "text-negative",
        )}
      >
        <span aria-hidden>{sign}</span>
        {sign === "+" && <span className="sr-only">{t("compare.row.add")}</span>}
        {sign === "−" && <span className="sr-only">{t("compare.row.remove")}</span>}
      </span>
      <SourceTag kind={item.source_kind} name={item.source_name} />
      <span className={cn("min-w-0 text-sm break-words", sign === "−" && "line-through")}>
        {item.title}
      </span>
    </li>
  );
}

type Props = {
  added: readonly Evidence[];
  removed: readonly Evidence[];
  kept: readonly Evidence[];
  onlyChanges: boolean;
};

/** Evidence gained (+) and lost (−) between two versions; unchanged ones only in the full view. */
export function EvidenceDiff({ added, removed, kept, onlyChanges }: Props) {
  const entries: Entry[] = [
    ...added.map((item): Entry => ({ item, sign: "+" })),
    ...removed.map((item): Entry => ({ item, sign: "−" })),
    ...(onlyChanges ? [] : kept.map((item): Entry => ({ item, sign: "" }))),
  ];
  return (
    <ul className="m-0 flex list-none flex-col gap-2 p-0">
      {entries.map((entry) => (
        <EvidenceRow key={`${entry.sign}-${entry.item.item_id}`} {...entry} />
      ))}
    </ul>
  );
}
