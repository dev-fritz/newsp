import { z } from "zod";

export const MIN_RETENTION = 7;
export const MAX_RETENTION = 3650;
export const MIN_INACTIVITY = 0;
export const MAX_INACTIVITY = 3650;
export const MIN_THRESHOLD = 0.5;
export const MAX_THRESHOLD = 0.99;

export const settingsFormSchema = z
  .object({
    embeddingModelId: z.string(),
    reportModelId: z.string(),
    ideaModelId: z.string(),
    retentionDays: z.string().trim(),
    threshold: z.string().trim(),
    signupEnabled: z.boolean(),
    inactivityDays: z.string().trim(),
  })
  .superRefine((v, ctx) => {
    const fail = (path: string, message: string) =>
      ctx.addIssue({ code: "custom", path: [path], message });
    const days = Number(v.retentionDays);
    if (!/^\d+$/.test(v.retentionDays) || days < MIN_RETENTION || days > MAX_RETENTION)
      fail("retentionDays", "invalid");
    const inactivity = Number(v.inactivityDays);
    if (
      !/^\d+$/.test(v.inactivityDays) ||
      inactivity < MIN_INACTIVITY ||
      inactivity > MAX_INACTIVITY
    )
      fail("inactivityDays", "invalid");
    const threshold = Number(v.threshold);
    if (!Number.isFinite(threshold) || threshold < MIN_THRESHOLD || threshold > MAX_THRESHOLD)
      fail("threshold", "invalid");
  });

export type SettingsFormValues = z.infer<typeof settingsFormSchema>;

/** API field names to the form's field names. */
export const SETTINGS_FIELD_MAP: Record<string, keyof SettingsFormValues> = {
  embedding_model_id: "embeddingModelId",
  default_report_model_id: "reportModelId",
  default_idea_model_id: "ideaModelId",
  retention_days: "retentionDays",
  idea_similarity_threshold: "threshold",
  signup_enabled: "signupEnabled",
  inactivity_pause_days: "inactivityDays",
};
