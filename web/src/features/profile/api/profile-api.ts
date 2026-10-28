import { getData, send } from "@/lib/http";
import type { components } from "@/types/api";

export type Preferences = components["schemas"]["dtos.PreferencesResponse"];
export type PreferencesRequest = components["schemas"]["dtos.PreferencesRequest"];
export type ChatModel = components["schemas"]["dtos.ChatModelResponse"];
export type SourceSummary = components["schemas"]["dtos.SourceSummary"];

type Schemas = components["schemas"];
type Needs<T, K extends keyof T> = Omit<T, K> & Required<Pick<T, K>>;

/** A chat model of one of the user's own providers. */
export type OwnModel = Needs<
  Schemas["dtos.ModelResponse"],
  "id" | "provider_id" | "name" | "display_name" | "enabled" | "tokens_last_7d"
>;

/** One of the user's own AI providers (bring your own key). */
export type OwnProvider = Omit<
  Needs<Schemas["dtos.ProviderResponse"], "id" | "name" | "kind" | "enabled" | "has_api_key">,
  "models"
> & { models: OwnModel[] };

export type OwnProviderKind = NonNullable<Schemas["dtos.CreateOwnProviderRequest"]["kind"]>;

export type OwnProviders = Omit<
  Needs<Schemas["dtos.OwnProvidersResponse"], "byok_enabled" | "providers">,
  "providers"
> & { providers: OwnProvider[] };

export type ConnectionTest = Needs<Schemas["dtos.ConnectionTestResponse"], "ok" | "models">;

export type CreateOwnProviderRequest = Schemas["dtos.CreateOwnProviderRequest"];
export type UpdateOwnProviderRequest = Schemas["dtos.UpdateOwnProviderRequest"];
export type CreateOwnModelRequest = Schemas["dtos.CreateOwnModelRequest"];
export type UpdateOwnModelRequest = Schemas["dtos.UpdateModelRequest"];

export function fetchPreferences(): Promise<Preferences> {
  return getData<Preferences>("/me/preferences");
}

export function savePreferences(body: PreferencesRequest): Promise<Preferences> {
  return send<Preferences>("put", "/me/preferences", body);
}

export function fetchChatModels(): Promise<ChatModel[]> {
  return getData<ChatModel[]>("/ai/models");
}

export function fetchSources(): Promise<SourceSummary[]> {
  return getData<SourceSummary[]>("/sources");
}

export function fetchOwnProviders(): Promise<OwnProviders> {
  return getData<OwnProviders>("/me/ai/providers");
}

export function createOwnProvider(body: CreateOwnProviderRequest): Promise<OwnProvider> {
  return send<OwnProvider>("post", "/me/ai/providers", body);
}

export function updateOwnProvider(
  id: string,
  body: UpdateOwnProviderRequest,
): Promise<OwnProvider> {
  return send<OwnProvider>("patch", `/me/ai/providers/${id}`, body);
}

export function deleteOwnProvider(id: string): Promise<void> {
  return send<void>("delete", `/me/ai/providers/${id}`);
}

export function testOwnProvider(id: string): Promise<ConnectionTest> {
  return send<ConnectionTest>("post", `/me/ai/providers/${id}/test`);
}

export function createOwnModel(providerId: string, body: CreateOwnModelRequest): Promise<OwnModel> {
  return send<OwnModel>("post", `/me/ai/providers/${providerId}/models`, body);
}

export function updateOwnModel(id: string, body: UpdateOwnModelRequest): Promise<OwnModel> {
  return send<OwnModel>("patch", `/me/ai/models/${id}`, body);
}

export function deleteOwnModel(id: string): Promise<void> {
  return send<void>("delete", `/me/ai/models/${id}`);
}
