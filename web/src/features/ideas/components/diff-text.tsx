import { useTranslation } from "react-i18next";

import type { DiffOp } from "../lib/diff";

/**
 * Text with its word-level changes marked. Colour is never the only cue: added
 * text is underlined, removed text is struck through, and both have screen reader text.
 */
export function DiffText({ ops }: { ops: readonly DiffOp[] }) {
  const { t } = useTranslation("ideas");
  return (
    <>
      {ops.map((op, position) => {
        const key = `${position}-${op.type}`;
        if (op.type === "add") {
          return (
            <ins
              key={key}
              className="rounded-[3px] bg-positive-soft px-0.5 text-foreground underline decoration-positive underline-offset-[3px]"
            >
              <span className="sr-only">{t("compare.srAdded")} </span>
              {op.text}
            </ins>
          );
        }
        if (op.type === "remove") {
          return (
            <del
              key={key}
              className="rounded-[3px] bg-negative-soft px-0.5 text-muted-foreground line-through decoration-negative"
            >
              <span className="sr-only">{t("compare.srRemoved")} </span>
              {op.text}
            </del>
          );
        }
        return <span key={key}>{op.text}</span>;
      })}
    </>
  );
}
