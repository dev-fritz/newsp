import { useMemo } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import type { Citation } from "../types";
import { CitationsContext } from "./citations-context";
import { articleComponents, calloutComponents } from "./markdown-elements";

const REMARK_PLUGINS = [remarkGfm];

/**
 * Splits the leading "### What changed…" block (rendered as a callout) from the
 * "## n. heading" sections that follow it.
 */
function splitCallout(markdown: string): { callout: string; body: string } {
  const text = markdown.trim();
  if (!text.startsWith("### ")) return { callout: "", body: text };
  const next = text.search(/^## /m);
  return next === -1
    ? { callout: text, body: "" }
    : { callout: text.slice(0, next).trim(), body: text.slice(next) };
}

/**
 * A report's Markdown in the serif reading style. GFM only, no raw HTML; `[n]`
 * citations become clickable chips that open the cited item.
 */
export function ReportMarkdown({
  content,
  citations,
}: {
  content: string;
  citations?: Citation[];
}) {
  const byRef = useMemo(() => {
    const map = new Map<number, Citation>();
    for (const citation of citations ?? []) {
      if (typeof citation.ref === "number") map.set(citation.ref, citation);
    }
    return map;
  }, [citations]);
  const { callout, body } = splitCallout(content);

  return (
    <CitationsContext value={byRef}>
      <div className="flex flex-col gap-6">
        {callout && (
          <section className="rounded-xl bg-muted px-4 py-3.5 sm:rounded-lg sm:px-[22px] sm:py-[18px]">
            <div className="flex flex-col gap-2.5">
              <ReactMarkdown remarkPlugins={REMARK_PLUGINS} components={calloutComponents}>
                {callout}
              </ReactMarkdown>
            </div>
          </section>
        )}
        {body && (
          <div className="flex flex-col gap-[14px] font-serif text-[17.5px] leading-[1.65] text-foreground sm:gap-[18px] sm:text-[19px]">
            <ReactMarkdown remarkPlugins={REMARK_PLUGINS} components={articleComponents}>
              {body}
            </ReactMarkdown>
          </div>
        )}
      </div>
    </CitationsContext>
  );
}
