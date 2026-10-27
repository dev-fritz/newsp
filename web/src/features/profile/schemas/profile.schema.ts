import { z } from "zod";

// Messages are i18n keys under common:fieldErrors.
export const preferencesSchema = z.object({
  topics: z.array(z.string().trim().min(1).max(60, { error: "max" })).max(20, { error: "max" }),
  output_lang: z.enum(["pt-BR", "en"]),
  timezone: z.string().min(1, { error: "required" }),
  report_enabled: z.boolean(),
  report_time: z.string().regex(/^([01]\d|2[0-3]):[0-5]\d$/, { error: "time" }),
  ideas_per_day: z.number().int().min(0).max(10),
  /** "" means the instance default. */
  report_model_id: z.string(),
  idea_model_id: z.string(),
  theme: z.enum(["light", "dark", "system"]),
  /** Checked sources; the API stores the excluded ones. */
  included_source_ids: z.array(z.string()),
  ui_locale: z.enum(["pt-BR", "en"]),
});
export type PreferencesValues = z.infer<typeof preferencesSchema>;

/** Shared by the form (client) and the route action (server). */
export const passwordSchema = z.object({
  current_password: z.string().min(1, { error: "required" }).max(72, { error: "max" }),
  new_password: z.string().min(12, { error: "password" }).max(72, { error: "password" }),
});

export const passwordFormSchema = passwordSchema
  .extend({ confirm: z.string() })
  .refine((v) => v.new_password === v.confirm, { path: ["confirm"], error: "mismatch" });
export type PasswordFormValues = z.infer<typeof passwordFormSchema>;

/** Field errors keyed by field name, from a failed parse. */
export function passwordFieldErrors(error: z.ZodError): Record<string, string> {
  const out: Record<string, string> = {};
  for (const issue of error.issues) {
    const key = issue.path.join(".");
    if (key && !out[key]) out[key] = issue.message;
  }
  return out;
}

export type PasswordActionResult = {
  done?: boolean;
  formError?: string;
  fields?: Record<string, string>;
};
