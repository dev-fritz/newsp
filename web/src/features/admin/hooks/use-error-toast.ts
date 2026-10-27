import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { isApiError } from "@/lib/api-error";

/** Returns a function that toasts a failure, translated by its error code. */
export function useErrorToast() {
  const { t } = useTranslation("admin");
  return (error: unknown) => {
    const code = isApiError(error) ? error.code : "UNKNOWN_ERROR";
    toast.error(
      t(`errors.${code}`, {
        defaultValue: t(`common:errors.${code}`, {
          defaultValue: t("common:errors.UNKNOWN_ERROR"),
        }),
      }),
    );
  };
}
