import type { Provider } from "../types";

export type ProviderStatus = "disabled" | "error" | "untested" | "connected";

/** Derives the card state from enabled / last_error / last_check_at. */
export function providerStatus(p: Provider): ProviderStatus {
  if (!p.enabled) return "disabled";
  if (p.last_error) return "error";
  if (!p.last_check_at) return "untested";
  return "connected";
}

/** Whether new models of this kind may be embeddings (Anthropic has none). */
export function supportsEmbeddings(p: Provider): boolean {
  return p.kind !== "anthropic";
}
