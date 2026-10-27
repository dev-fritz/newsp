import { useTranslation } from "react-i18next";

import { windowHours } from "../format";

/** Returns "últimas 6 h" / "últimos 7 dias" for the window a report covers. */
export function useWindowLabel(): (start: string | undefined, end: string | undefined) => string {
  const { t } = useTranslation("reports");
  return (start, end) => {
    const hours = windowHours(start, end);
    return t([`window.h${hours}`, "window.custom"], { hours });
  };
}
