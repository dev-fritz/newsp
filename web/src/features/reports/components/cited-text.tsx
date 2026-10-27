import { Children, type ReactNode } from "react";

import { CitationChip } from "./citation-chip";

const CITATION = /\[(\d{1,3})\]/g;

/** Splits a text node on `[n]` and renders each reference as a chip (never HTML). */
function CitedString({ text }: { text: string }) {
  const parts = text.split(CITATION);
  if (parts.length === 1) return <>{text}</>;
  return (
    <>
      {parts.map((part, i) =>
        // split() with one capture group alternates text (even) and reference (odd).
        i % 2 === 1 ? (
          <CitationChip key={`ref-${i}-${part}`} reference={Number(part)} />
        ) : (
          part && <span key={`text-${i}`}>{part}</span>
        ),
      )}
    </>
  );
}

/** Children of an inline-level element, with `[n]` text turned into citation chips. */
export function CitedText({ children }: { children?: ReactNode }) {
  return (
    <>
      {Children.map(children, (child) =>
        typeof child === "string" ? <CitedString text={child} /> : child,
      )}
    </>
  );
}
