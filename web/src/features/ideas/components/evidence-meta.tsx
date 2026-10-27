import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import { useIdeaFormat } from "../hooks/use-idea-format";
import type { Evidence } from "../schemas/idea-response.schema";

/** "Hacker News · 412 pontos · 88 comentários · 25 out." for an evidence item. */
export function EvidenceMeta({ item, className }: { item: Evidence; className?: string }) {
  const { t } = useTranslation("ideas");
  const format = useIdeaFormat();
  const parts = [
    item.source_name,
    item.score !== null ? t("evidence.score", { count: item.score }) : "",
    item.comments !== null ? t("evidence.comments", { count: item.comments }) : "",
    item.published_at ? format.date(item.published_at) : "",
  ].filter((part) => part !== "");
  return (
    <span className={cn("text-[12.5px] text-subtle-foreground", className)}>
      {parts.join(" · ")}
    </span>
  );
}
