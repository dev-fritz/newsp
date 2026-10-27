/** What auth actions return when they do not redirect. */
export type AuthActionResult = {
  formError?: string; // error code, translated by the page
  fields?: Record<string, string>; // field -> i18n key under common:fieldErrors
  done?: boolean;
};
