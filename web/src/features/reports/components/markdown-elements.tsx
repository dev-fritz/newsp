import type { Components } from "react-markdown";

import { cn } from "@/lib/utils";

import { safeHttpUrl } from "../format";
import { CitedText } from "./cited-text";

/** Element renderers shared by the article and the "what changed" callout. */
const shared: Components = {
  p: ({ children }) => (
    <p className="text-pretty">
      <CitedText>{children}</CitedText>
    </p>
  ),
  strong: ({ children }) => (
    <strong className="font-semibold">
      <CitedText>{children}</CitedText>
    </strong>
  ),
  em: ({ children }) => (
    <em>
      <CitedText>{children}</CitedText>
    </em>
  ),
  del: ({ children }) => (
    <del>
      <CitedText>{children}</CitedText>
    </del>
  ),
  a: ({ href, children }) => {
    const safe = safeHttpUrl(href);
    if (!safe) return <span>{children}</span>;
    return (
      <a
        href={safe}
        target="_blank"
        rel="noopener noreferrer"
        className="text-signal underline underline-offset-2"
      >
        {children}
      </a>
    );
  },
  // Remote images are not loaded from generated content.
  img: () => null,
  hr: () => <hr className="border-border" />,
  code: ({ children, className }) => (
    <code className={cn("rounded bg-muted px-1 py-0.5 font-mono text-[0.85em]", className)}>
      {children}
    </code>
  ),
  pre: ({ children }) => (
    <pre className="overflow-x-auto rounded-lg bg-muted p-3 font-mono text-sm">{children}</pre>
  ),
  blockquote: ({ children }) => (
    <blockquote className="flex flex-col gap-3 border-l-2 border-border-strong pl-4 text-muted-foreground">
      {children}
    </blockquote>
  ),
  table: ({ children }) => (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-left font-sans text-sm">{children}</table>
    </div>
  ),
  th: ({ children }) => (
    <th className="border-b border-border-strong px-2 py-1.5 font-semibold">
      <CitedText>{children}</CitedText>
    </th>
  ),
  td: ({ children }) => (
    <td className="border-b border-border px-2 py-1.5">
      <CitedText>{children}</CitedText>
    </td>
  ),
};

/** The article body: "## n. heading" sections in a serif reading style. */
export const articleComponents: Components = {
  ...shared,
  h1: ({ children }) => <h2 className="font-serif text-2xl font-semibold">{children}</h2>,
  h2: ({ children }) => (
    <h2 className="mt-4 font-serif text-[23px] leading-tight font-semibold text-balance first:mt-0 sm:text-[27px]">
      {children}
    </h2>
  ),
  h3: ({ children }) => <h3 className="font-serif text-xl font-semibold">{children}</h3>,
  ul: ({ children }) => <ul className="flex list-disc flex-col gap-1.5 pl-6">{children}</ul>,
  ol: ({ children }) => <ol className="flex list-decimal flex-col gap-1.5 pl-6">{children}</ol>,
  li: ({ children }) => (
    <li>
      <CitedText>{children}</CitedText>
    </li>
  ),
};

/** The sunken "what changed" box: a kicker title and a short bullet list in sans. */
export const calloutComponents: Components = {
  ...shared,
  h3: ({ children }) => <h2 className="kicker text-foreground">{children}</h2>,
  ul: ({ children }) => (
    <ul className="flex list-disc flex-col gap-1.5 pl-[18px] font-sans text-[14.5px] leading-normal sm:text-[15px]">
      {children}
    </ul>
  ),
  li: ({ children }) => (
    <li>
      <CitedText>{children}</CitedText>
    </li>
  ),
};
