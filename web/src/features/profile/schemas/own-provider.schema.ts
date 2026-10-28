import { z } from "zod";

import type { OwnProviderKind } from "@/features/profile/api/profile-api";

/** Hosted kinds a user can connect, in display order. The base URL is fixed by the server. */
export const OWN_PROVIDER_KINDS: readonly OwnProviderKind[] = ["openai", "anthropic", "gemini"];

// Messages are i18n keys under common:fieldErrors.
export const ownKeySchema = z.object({
  api_key: z.string().trim().min(1, { error: "required" }).max(500, { error: "max" }),
});
export type OwnKeyValues = z.infer<typeof ownKeySchema>;
