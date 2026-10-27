import type { MetaDescriptor } from "react-router";

import { DEFAULT_LOCALE, fixedT, isLocale } from "@/i18n";

type TitleKey =
  | "home"
  | "reports"
  | "report"
  | "ideas"
  | "idea"
  | "ideaCompare"
  | "feed"
  | "profile"
  | "sources"
  | "ai"
  | "users"
  | "login"
  | "setup"
  | "setupAi"
  | "setupSources"
  | "invite"
  | "resetPassword"
  | "signup"
  | "verifyEmail"
  | "forgotPassword"
  | "notFound";

type Match = { id: string; loaderData: unknown } | undefined;

/** `<page> - newsp`, in the locale the root loader picked for this request. */
export function pageTitle(matches: readonly Match[], key: TitleKey): MetaDescriptor[] {
  const root = matches.find((match) => match?.id === "root")?.loaderData;
  const locale =
    typeof root === "object" && root !== null && "locale" in root && isLocale(root.locale)
      ? root.locale
      : DEFAULT_LOCALE;
  return [{ title: `${fixedT(locale)(`titles.${key}`)} - newsp` }];
}
