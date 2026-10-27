import { useTranslation } from "react-i18next";

/** "2 novas, 1 atualizada" for a finished generation. */
export function useOutcomeSummary(): (ids: { created: string[]; merged: string[] }) => string {
  const { t } = useTranslation("ideas");
  return ({ created, merged }) => {
    const parts = [
      created.length > 0 && t("generate.created", { count: created.length }),
      merged.length > 0 && t("generate.updated", { count: merged.length }),
    ].filter((part): part is string => Boolean(part));
    return parts.length > 0 ? parts.join(", ") : t("dialog.none");
  };
}
