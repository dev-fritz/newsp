import { createContext } from "react";

import type { Citation } from "../types";

/** The citations of the report being rendered, by their `[n]` reference. */
export const CitationsContext = createContext<ReadonlyMap<number, Citation>>(new Map());
