import { createContext, useContext } from "react";

import type { Evidence } from "../schemas/idea-response.schema";

/** Evidence of the idea being read, by ref ("E1"), so citations only link and preview real entries. */
const CitationContext = createContext<ReadonlyMap<string, Evidence>>(new Map());

export const CitationProvider = CitationContext.Provider;

export function useCitedEvidence(): ReadonlyMap<string, Evidence> {
  return useContext(CitationContext);
}
