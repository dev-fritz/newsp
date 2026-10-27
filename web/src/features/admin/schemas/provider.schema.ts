import { z } from "zod";

import { PROVIDER_KINDS } from "../lib/provider-kinds";
import { isHttpUrl } from "./source.schema";

/**
 * Provider form. The key is optional for local servers; a hosted provider
 * needs one when it is created. Messages are i18n keys under common:fieldErrors.
 */
export function buildProviderSchema(requireKey: boolean) {
  return z
    .object({
      kind: z.enum(PROVIDER_KINDS),
      name: z.string().trim().min(1, { error: "required" }).max(60, { error: "max" }),
      baseUrl: z.string().trim().max(500, { error: "max" }),
      apiKey: z.string().trim().max(500, { error: "max" }),
    })
    .superRefine((v, ctx) => {
      const fail = (path: string, message: string) =>
        ctx.addIssue({ code: "custom", path: [path], message });
      if (v.kind === "openai_compatible" && !v.baseUrl) fail("baseUrl", "required");
      else if (v.baseUrl && !isHttpUrl(v.baseUrl)) fail("baseUrl", "url");
      if (requireKey && v.kind !== "openai_compatible" && !v.apiKey) fail("apiKey", "required");
    });
}

export type ProviderFormValues = z.infer<ReturnType<typeof buildProviderSchema>>;

/** API field names to the form's field names. */
export const PROVIDER_FIELD_MAP: Record<string, keyof ProviderFormValues> = {
  name: "name",
  kind: "kind",
  base_url: "baseUrl",
  api_key: "apiKey",
};
