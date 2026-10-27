import type {
  Preferences,
  PreferencesRequest,
  SourceSummary,
} from "@/features/profile/api/profile-api";
import type { PreferencesValues } from "@/features/profile/schemas/profile.schema";

export const DEFAULT_MODEL = "";

/** Zones offered besides the browser's and the saved one. */
export const COMMON_TIMEZONES = [
  "UTC",
  "America/Sao_Paulo",
  "America/Manaus",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/Mexico_City",
  "America/Argentina/Buenos_Aires",
  "Europe/Lisbon",
  "Europe/London",
  "Europe/Berlin",
  "Europe/Paris",
  "Africa/Johannesburg",
  "Asia/Dubai",
  "Asia/Kolkata",
  "Asia/Singapore",
  "Asia/Tokyo",
  "Australia/Sydney",
] as const;

/** Form values from the API's preferences (sources become the *included* ones). */
export function toFormValues(prefs: Preferences, sources: SourceSummary[]): PreferencesValues {
  const excluded = new Set(prefs.excluded_source_ids ?? []);
  return {
    topics: prefs.topics ?? [],
    output_lang: prefs.output_lang ?? "pt-BR",
    timezone: prefs.timezone ?? "UTC",
    report_enabled: prefs.report_enabled ?? true,
    report_time: prefs.report_time ?? "08:00",
    ideas_per_day: prefs.ideas_per_day ?? 3,
    report_model_id: prefs.report_model_id || DEFAULT_MODEL,
    idea_model_id: prefs.idea_model_id || DEFAULT_MODEL,
    theme: prefs.theme ?? "system",
    included_source_ids: sources.flatMap((s) => (s.id && !excluded.has(s.id) ? [s.id] : [])),
    ui_locale: prefs.ui_locale ?? "pt-BR",
  };
}

/**
 * The API stores excluded ids (full replace). Ids the user cannot see in the
 * list (e.g. a disabled source) keep their previous exclusion.
 */
export function toRequest(
  values: PreferencesValues,
  prefs: Preferences,
  sources: SourceSummary[],
): PreferencesRequest {
  const listed = new Set(sources.flatMap((s) => (s.id ? [s.id] : [])));
  const included = new Set(values.included_source_ids);
  const hidden = (prefs.excluded_source_ids ?? []).filter((id) => !listed.has(id));
  const excluded = [...listed].filter((id) => !included.has(id));
  return {
    topics: values.topics,
    output_lang: values.output_lang,
    timezone: values.timezone,
    report_enabled: values.report_enabled,
    report_time: values.report_time,
    ideas_per_day: values.ideas_per_day,
    // The API treats an absent model as "instance default".
    report_model_id: values.report_model_id || undefined,
    idea_model_id: values.idea_model_id || undefined,
    theme: values.theme,
    excluded_source_ids: [...hidden, ...excluded],
    ui_locale: values.ui_locale,
  };
}
