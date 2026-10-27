import { getData, send } from "@/lib/http";

import type { JobAccepted, Source, SourceKind } from "../types";

export type CreateSourceBody = {
  kind: SourceKind;
  name: string;
  interval_minutes: number;
  enabled?: boolean;
  config: Record<string, unknown>;
  /** Credential of kinds that take one; encrypted by the API and never returned. */
  secret?: string;
};

export type UpdateSourceBody = {
  name?: string;
  interval_minutes?: number;
  enabled?: boolean;
  config?: Record<string, unknown>;
  /** Replaces the credential; an empty string removes it. */
  secret?: string;
};

export function listSources(): Promise<Source[]> {
  return getData<Source[]>("/admin/sources");
}

export function createSource(body: CreateSourceBody): Promise<Source> {
  return send<Source>("post", "/admin/sources", body);
}

export function updateSource(id: string, body: UpdateSourceBody): Promise<Source> {
  return send<Source>("patch", `/admin/sources/${encodeURIComponent(id)}`, body);
}

export function deleteSource(id: string): Promise<void> {
  return send<void>("delete", `/admin/sources/${encodeURIComponent(id)}`);
}

export function collectSource(id: string): Promise<JobAccepted> {
  return send<JobAccepted>("post", `/admin/sources/${encodeURIComponent(id)}/collect`);
}
