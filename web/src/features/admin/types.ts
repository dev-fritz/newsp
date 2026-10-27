import type { components } from "@/types/api";

type Schemas = components["schemas"];
type Needs<T, K extends keyof T> = Omit<T, K> & Required<Pick<T, K>>;

/** Fields the API always sends are required here; the generated types mark everything optional. */
export type Source = Omit<
  Needs<
    Schemas["dtos.SourceResponse"],
    "id" | "kind" | "name" | "enabled" | "interval_minutes" | "items_24h"
  >,
  "config"
> & { config: Record<string, unknown> };

export type SourceKind = Source["kind"];

export type AiModel = Needs<
  Schemas["dtos.ModelResponse"],
  "id" | "provider_id" | "name" | "display_name" | "capability" | "enabled" | "tokens_last_7d"
>;

export type Capability = AiModel["capability"];

export type Provider = Omit<
  Needs<Schemas["dtos.ProviderResponse"], "id" | "name" | "kind" | "enabled">,
  "models"
> & { models: AiModel[] };

export type ProviderKind = Provider["kind"];

export type ConnectionTest = { ok: boolean; models: string[] | null; error?: string };

export type InstanceSettings = Schemas["dtos.SettingsResponse"];

export type AdminUser = Needs<
  Schemas["dtos.UserResponse"],
  "id" | "name" | "email" | "role" | "status" | "email_verified"
>;

export type Role = AdminUser["role"];

export type Invite = Needs<
  Schemas["dtos.InviteResponse"],
  "id" | "role" | "created_at" | "expires_at"
>;

export type OneTimeLink = Needs<Schemas["dtos.LinkResponse"], "url" | "expires_at">;

export type InviteLink = Needs<
  Schemas["dtos.InviteLinkResponse"],
  "url" | "expires_at" | "emailed"
>;

export type Job = Needs<Schemas["dtos.JobResponse"], "id" | "status">;

export type JobAccepted = { job_id: string };
