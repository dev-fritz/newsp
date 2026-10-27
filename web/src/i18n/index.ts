import { createInstance, type i18n as I18n, type Resource, type TFunction } from "i18next";
import { initReactI18next } from "react-i18next";

export const LOCALES = ["pt-BR", "en"] as const;
export type Locale = (typeof LOCALES)[number];
export const DEFAULT_LOCALE: Locale = "pt-BR";

export function isLocale(value: unknown): value is Locale {
  return typeof value === "string" && (LOCALES as readonly string[]).includes(value);
}

// Shared strings live in i18n/locales/<lng>/<ns>.json; each feature owns
// features/<feature>/locales/<lng>.json as the namespace "<feature>".
const shared = import.meta.glob<{ default: Record<string, unknown> }>("./locales/*/*.json", {
  eager: true,
});
const features = import.meta.glob<{ default: Record<string, unknown> }>(
  "../features/*/locales/*.json",
  { eager: true },
);

function buildResources(): Resource {
  const resources: Resource = {};
  for (const [path, mod] of Object.entries(shared)) {
    const [, lng, ns] = path.match(/\.\/locales\/([^/]+)\/([^/]+)\.json$/) ?? [];
    if (lng && ns) (resources[lng] ??= {})[ns] = mod.default;
  }
  for (const [path, mod] of Object.entries(features)) {
    const [, ns, lng] = path.match(/\.\.\/features\/([^/]+)\/locales\/([^/]+)\.json$/) ?? [];
    if (lng && ns) (resources[lng] ??= {})[ns] = mod.default;
  }
  return resources;
}

const resources = buildResources();

/** A fresh instance per render, so concurrent server requests never share a language. */
export function createI18n(lng: Locale): I18n {
  const instance = createInstance();
  void instance.use(initReactI18next).init({
    resources,
    lng,
    fallbackLng: DEFAULT_LOCALE,
    defaultNS: "common",
    interpolation: { escapeValue: false }, // React escapes
    initAsync: false,
  });
  return instance;
}

// One shared instance for code outside React (route `meta`). getFixedT pins the
// language per call, so this never leaks a language across requests.
const plain = createInstance();
void plain.init({
  resources,
  fallbackLng: DEFAULT_LOCALE,
  defaultNS: "common",
  interpolation: { escapeValue: false },
  initAsync: false,
});

/** A translator bound to `lng`, for code without the React i18n context. */
export function fixedT(lng: Locale): TFunction {
  return plain.getFixedT(lng);
}

/** Picks a supported locale from an Accept-Language header. */
export function localeFromHeader(header: string | null): Locale {
  if (!header) return DEFAULT_LOCALE;
  for (const part of header.split(",")) {
    const tag = part.split(";")[0]?.trim().toLowerCase() ?? "";
    if (tag.startsWith("pt")) return "pt-BR";
    if (tag.startsWith("en")) return "en";
  }
  return DEFAULT_LOCALE;
}
