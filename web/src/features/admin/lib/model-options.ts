import type { Capability, Provider } from "../types";

export type ModelOption = { value: string; label: string };

/** Enabled models of enabled providers with the given capability, labeled "name · provider". */
export function modelOptions(providers: Provider[], capability: Capability): ModelOption[] {
  return providers
    .filter((p) => p.enabled)
    .flatMap((p) =>
      p.models
        .filter((m) => m.enabled && m.capability === capability)
        .map((m) => ({ value: m.id, label: `${m.display_name || m.name} · ${p.name}` })),
    );
}
