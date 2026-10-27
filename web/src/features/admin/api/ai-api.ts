import { getData, send } from "@/lib/http";

import type {
  AiModel,
  Capability,
  ConnectionTest,
  InstanceSettings,
  Provider,
  ProviderKind,
} from "../types";

export type CreateProviderBody = {
  name: string;
  kind: ProviderKind;
  base_url?: string;
  api_key?: string;
};

/** api_key: omit to keep the current key, "" removes it. */
export type UpdateProviderBody = {
  name?: string;
  base_url?: string;
  api_key?: string;
  enabled?: boolean;
};

export type CreateModelBody = { name: string; capability: Capability; display_name?: string };
export type UpdateModelBody = { display_name?: string; enabled?: boolean };

/** Model ids accept "" to clear. */
export type UpdateSettingsBody = {
  embedding_model_id?: string;
  default_report_model_id?: string;
  default_idea_model_id?: string;
  retention_days?: number;
  idea_similarity_threshold?: number;
  signup_enabled?: boolean;
  inactivity_pause_days?: number;
};

const id = encodeURIComponent;

export function listProviders(): Promise<Provider[]> {
  return getData<Provider[]>("/admin/ai/providers");
}

export function createProvider(body: CreateProviderBody): Promise<Provider> {
  return send<Provider>("post", "/admin/ai/providers", body);
}

export function updateProvider(providerId: string, body: UpdateProviderBody): Promise<Provider> {
  return send<Provider>("patch", `/admin/ai/providers/${id(providerId)}`, body);
}

export function deleteProvider(providerId: string): Promise<void> {
  return send<void>("delete", `/admin/ai/providers/${id(providerId)}`);
}

export function testProvider(providerId: string): Promise<ConnectionTest> {
  return send<ConnectionTest>("post", `/admin/ai/providers/${id(providerId)}/test`);
}

export function createModel(providerId: string, body: CreateModelBody): Promise<AiModel> {
  return send<AiModel>("post", `/admin/ai/providers/${id(providerId)}/models`, body);
}

export function updateModel(modelId: string, body: UpdateModelBody): Promise<AiModel> {
  return send<AiModel>("patch", `/admin/ai/models/${id(modelId)}`, body);
}

export function deleteModel(modelId: string): Promise<void> {
  return send<void>("delete", `/admin/ai/models/${id(modelId)}`);
}

export function getSettings(): Promise<InstanceSettings> {
  return getData<InstanceSettings>("/admin/settings");
}

export function updateSettings(body: UpdateSettingsBody): Promise<InstanceSettings> {
  return send<InstanceSettings>("patch", "/admin/settings", body);
}
