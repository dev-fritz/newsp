import ReactMarkdown, { type Components } from "react-markdown";
import { useTranslation } from "react-i18next";
import remarkGfm from "remark-gfm";

import { cn } from "@/lib/utils";

import { CITE_HASH_PREFIX, isHttpUrl, linkCitations } from "../lib/idea-text";
import { useCitedEvidence } from "./citation-context";
import { CiteChip } from "./cite-chip";
import { EvidenceCite } from "./evidence-cite";

function MarkdownLink({ href, children }: React.ComponentProps<"a">) {
  const { t } = useTranslation("ideas");
  const evidence = useCitedEvidence();
  if (href?.startsWith(CITE_HASH_PREFIX)) {
    const ref = href.slice(CITE_HASH_PREFIX.length);
    const item = evidence.get(ref);
    if (item) return <EvidenceCite item={item} />;
    return <CiteChip refId={ref} label={t("evidence.cite", { ref })} />;
  }
  if (href && (isHttpUrl(href) || href.startsWith("mailto:"))) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        className="text-signal underline underline-offset-2"
      >
        {children}
      </a>
    );
  }
  return <span>{children}</span>;
}

const components: Components = {
  p: ({ children }) => <p className="[&:not(:first-child)]:mt-4">{children}</p>,
  a: MarkdownLink,
  strong: ({ children }) => <strong className="font-semibold">{children}</strong>,
  ul: ({ children }) => <ul className="my-3 list-disc space-y-1.5 pl-6">{children}</ul>,
  ol: ({ children }) => <ol className="my-3 list-decimal space-y-1.5 pl-6">{children}</ol>,
  h1: ({ children }) => <h3 className="mt-5 text-xl font-semibold">{children}</h3>,
  h2: ({ children }) => <h3 className="mt-5 text-xl font-semibold">{children}</h3>,
  h3: ({ children }) => <h3 className="mt-5 text-xl font-semibold">{children}</h3>,
  h4: ({ children }) => <h4 className="mt-4 text-lg font-semibold">{children}</h4>,
  blockquote: ({ children }) => (
    <blockquote className="my-3 border-l-2 border-border-strong pl-4 text-muted-foreground italic">
      {children}
    </blockquote>
  ),
  code: ({ children }) => (
    <code className="rounded bg-muted px-1 py-0.5 font-mono text-[0.85em]">{children}</code>
  ),
  pre: ({ children }) => (
    <pre className="my-3 overflow-x-auto rounded-lg bg-muted p-3 font-mono text-sm">{children}</pre>
  ),
  table: ({ children }) => (
    <div className="my-3 overflow-x-auto">
      <table className="w-full border-collapse text-left font-sans text-sm">{children}</table>
    </div>
  ),
  th: ({ children }) => (
    <th className="border-b border-border-strong px-2 py-1.5 font-medium">{children}</th>
  ),
  td: ({ children }) => <td className="border-b border-border px-2 py-1.5">{children}</td>,
};

type MarkdownTextProps = { value: string; className?: string };

/** Markdown from the API (no raw HTML), with "[E1]" citations drawn as chips. */
export function MarkdownText({ value, className }: MarkdownTextProps) {
  return (
    <div
      className={cn(
        "max-w-[75ch] font-serif text-[17.5px] leading-[1.65] text-pretty break-words sm:text-lg",
        className,
      )}
    >
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {linkCitations(value)}
      </ReactMarkdown>
    </div>
  );
}
