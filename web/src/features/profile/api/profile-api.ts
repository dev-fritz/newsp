import { getData, send } from "@/lib/http";
import type { components } from "@/types/api";

export type Preferences = components["schemas"]["dtos.PreferencesResponse"];
export type PreferencesRequest = components["schemas"]["dtos.PreferencesRequest"];
export type ChatModel = components["schemas"]["dtos.ChatModelResponse"];
export type SourceSummary = components["schemas"]["dtos.SourceSummary"];

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
