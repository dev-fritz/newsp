import { useTranslation } from "react-i18next";

import type { IdeaListItem } from "../schemas/idea-response.schema";
import { IdeaBadge } from "./idea-badge";

type Props = Pick<IdeaListItem, "status" | "current_version" | "last_origin">;

/** Status of an idea in a list: discarded, updated/edited versions, new or saved. */
export function IdeaStatusBadge({ status, current_version, last_origin }: Props) {
  const { t } = useTranslation("ideas");
  if (status === "discarded") return <IdeaBadge tone="outline">{t("status.discarded")}</IdeaBadge>;
  if (current_version > 1 && last_origin !== "generated") {
    const key = last_origin === "manual" ? "badge.edited" : "badge.updated";
    return <IdeaBadge tone="signal">{t(key, { version: current_version })}</IdeaBadge>;
  }
  if (status === "new") return <IdeaBadge tone="marker">{t("status.new")}</IdeaBadge>;
  return <IdeaBadge>{t("status.saved")}</IdeaBadge>;
}
