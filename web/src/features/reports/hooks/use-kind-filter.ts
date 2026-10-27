import { useCallback } from "react";
import { useSearchParams } from "react-router";

import { isReportKind, type ReportKind } from "../types";

/** The list filter lives in the URL (`?kind=`), so it is shareable and survives navigation. */
export function useKindFilter(): [ReportKind | undefined, (kind: ReportKind | undefined) => void] {
  const [params, setParams] = useSearchParams();
  const raw = params.get("kind");
  const kind = isReportKind(raw) ? raw : undefined;
  const setKind = useCallback(
    (next: ReportKind | undefined) => {
      setParams(
        (prev) => {
          const copy = new URLSearchParams(prev);
          if (next) copy.set("kind", next);
          else copy.delete("kind");
          return copy;
        },
        { replace: true },
      );
    },
    [setParams],
  );
  return [kind, setKind];
}
