import { useTranslation } from "react-i18next";

import { Popover, PopoverAnchor, PopoverContent } from "@/components/ui/popover";
import { useHoverOpen } from "@/hooks/use-hover-open";

import { evidenceDomId, isHttpUrl } from "../lib/idea-text";
import type { Evidence } from "../schemas/idea-response.schema";
import { CiteChip } from "./cite-chip";
import { EvidenceMeta } from "./evidence-meta";
import { SourceTag } from "./source-tag";

/**
 * An "[E1]" citation in the idea's text: previews the evidence on hover or
 * keyboard focus, and jumps to its entry in the evidence list on click.
 */
export function EvidenceCite({ item }: { item: Evidence }) {
  const { t } = useTranslation("ideas");
  const { open, setOpen, triggerProps, contentProps } = useHoverOpen();
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverAnchor asChild>
        <CiteChip
          refId={item.ref}
          label={t("evidence.cite", { ref: item.ref })}
          href={`#${evidenceDomId(item.ref)}`}
          onPointerEnter={triggerProps.onPointerEnter}
          onPointerLeave={triggerProps.onPointerLeave}
          onFocus={triggerProps.onFocus}
          onBlur={triggerProps.onBlur}
        />
      </PopoverAnchor>
      <PopoverContent
        align="start"
        className="w-[340px] max-w-[calc(100vw-2rem)] gap-2.5 p-4 font-sans"
        {...contentProps}
      >
        <span className="flex min-w-0 items-center gap-2">
          <SourceTag kind={item.source_kind} name={item.source_name} />
          <EvidenceMeta item={item} className="min-w-0 truncate" />
        </span>
        <p className="text-sm leading-snug font-semibold text-foreground">{item.title}</p>
        {item.excerpt && (
          <p className="line-clamp-4 text-[13px] leading-[1.45] text-muted-foreground">
            {t("evidence.excerpt", { text: item.excerpt })}
          </p>
        )}
        {isHttpUrl(item.url) && (
          <a
            href={item.url}
            target="_blank"
            rel="noopener noreferrer"
            className="w-fit rounded-sm text-[13px] font-medium text-signal hover:underline focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
          >
            {t("evidence.openOriginal")}
          </a>
        )}
      </PopoverContent>
    </Popover>
  );
}
