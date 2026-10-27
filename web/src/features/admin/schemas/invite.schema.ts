import { z } from "zod";

export const MAX_EMAIL = 254;

/** Optional e-mail: empty is allowed; the API only gets it when filled. */
export const inviteFormSchema = z.object({
  email: z
    .string()
    .trim()
    .max(MAX_EMAIL, "max")
    .refine((v) => v === "" || z.email().safeParse(v).success, "email"),
});

export type InviteFormValues = z.infer<typeof inviteFormSchema>;
