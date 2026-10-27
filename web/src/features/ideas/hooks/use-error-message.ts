import { useTranslation } from "react-i18next";

/** Translates an error code: ideas namespace first, then the shared ones. */
export function useErrorMessage(): (code: string) => string {
  const { t } = useTranslation(["ideas", "common"]);
  return (code) =>
    t(`ideas:errors.${code}`, {
      defaultValue: t(`common:errors.${code}`, { defaultValue: t("common:errors.UNKNOWN_ERROR") }),
    });
}
