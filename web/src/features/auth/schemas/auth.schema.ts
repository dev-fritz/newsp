import { z } from "zod";

// Shared by the forms (client) and the actions (server). Messages are i18n
// keys under common:fieldErrors.
const email = z.email({ error: "email" }).max(254, { error: "max" });
const password = z.string().min(12, { error: "password" }).max(72, { error: "password" });

export const loginSchema = z.object({
  email: z.string().min(1, { error: "required" }).max(254, { error: "max" }),
  password: z.string().min(1, { error: "required" }).max(72, { error: "max" }),
  redirect: z.string().optional(),
});
export type LoginValues = z.infer<typeof loginSchema>;

export const accountSchema = z
  .object({
    name: z.string().trim().min(1, { error: "required" }).max(100, { error: "max" }),
    email,
    password,
    confirm: z.string(),
  })
  .refine((v) => v.password === v.confirm, { path: ["confirm"], error: "mismatch" });
export type AccountValues = z.infer<typeof accountSchema>;

export const resetSchema = z
  .object({ password, confirm: z.string() })
  .refine((v) => v.password === v.confirm, { path: ["confirm"], error: "mismatch" });
export type ResetValues = z.infer<typeof resetSchema>;

export const forgotSchema = z.object({ email });
export type ForgotValues = z.infer<typeof forgotSchema>;

/** Field errors keyed by field name, from a failed parse. */
export function fieldErrors(error: z.ZodError): Record<string, string> {
  const out: Record<string, string> = {};
  for (const issue of error.issues) {
    const key = issue.path.join(".");
    if (key && !out[key]) out[key] = issue.message;
  }
  return out;
}
