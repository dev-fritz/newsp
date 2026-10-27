import { z } from "zod";

import { getData, getPage, send, type PageMeta } from "@/lib/http";
import type { ApiError } from "@/lib/api-error";
import type { components } from "@/types/api";

import {
  generationResultSchema,
  ideaListItemSchema,
  ideaSchema,
  ideaVersionSchema,
  type GenerationResult,
  type Idea,
  type IdeaListItem,
  type IdeaStatus,
  type IdeaVersion,
} from "../schemas/idea-response.schema";
import type { IdeaStatusFilter } from "../schemas/idea.schema";

export type EditIdeaRequest = components["schemas"]["dtos.EditIdeaRequest"];
export type GenerateIdeasRequest = components["schemas"]["dtos.GenerateIdeasRequest"];
export type DeepenIdeaRequest = components["schemas"]["dtos.DeepenIdeaRequest"];

export type IdeaListQuery = { status: IdeaStatusFilter; q: string; limit: number; offset: number };
export type IdeaPage = { data: IdeaListItem[]; meta: PageMeta };

export type JobStatus = "pending" | "running" | "done" | "failed" | "canceled";
export type IdeaJob = { id: string; status: JobStatus; error: string | null; result?: unknown };

function malformed(): ApiError {
  return { status: 0, code: "UNKNOWN_ERROR", message: "unexpected response" };
}

function parse<T>(schema: z.ZodType<T>, value: unknown): T {
  const parsed = schema.safeParse(value);
  if (!parsed.success) throw malformed();
  return parsed.data;
}

export async function fetchIdeas(query: IdeaListQuery): Promise<IdeaPage> {
  const page = await getPage<unknown>("/ideas", {
    status: query.status,
    q: query.q || undefined,
    limit: query.limit,
    offset: query.offset,
  });
  return { data: parse(z.array(ideaListItemSchema), page.data), meta: page.meta };
}

export async function fetchIdea(id: string): Promise<Idea> {
  return parse(ideaSchema, await getData<unknown>(`/ideas/${encodeURIComponent(id)}`));
}

export async function fetchIdeaVersion(id: string, version: number): Promise<IdeaVersion> {
  return parse(
    ideaVersionSchema,
    await getData<unknown>(`/ideas/${encodeURIComponent(id)}/versions/${version}`),
  );
}

export async function updateIdeaStatus(id: string, status: IdeaStatus): Promise<void> {
  await send<void>("patch", `/ideas/${encodeURIComponent(id)}/status`, { status });
}

export async function editIdea(id: string, body: EditIdeaRequest): Promise<Idea> {
  return parse(ideaSchema, await send<unknown>("put", `/ideas/${encodeURIComponent(id)}`, body));
}

const jobAcceptedSchema = z.object({ job_id: z.string().min(1) });

export async function generateIdeas(body: GenerateIdeasRequest): Promise<string> {
  const accepted = parse(jobAcceptedSchema, await send<unknown>("post", "/ideas/generate", body));
  return accepted.job_id;
}

export async function deepenIdea(id: string, body: DeepenIdeaRequest): Promise<string> {
  const accepted = parse(
    jobAcceptedSchema,
    await send<unknown>("post", `/ideas/${encodeURIComponent(id)}/deepen`, body),
  );
  return accepted.job_id;
}

const jobSchema = z.object({
  id: z.string(),
  status: z.enum(["pending", "running", "done", "failed", "canceled"]),
  error: z
    .string()
    .nullish()
    .transform((v) => v || null),
  // Absent until the job finishes (the API omits it while pending or running).
  result: z.unknown().optional(),
});

export async function fetchJob(id: string): Promise<IdeaJob> {
  return parse(jobSchema, await getData<unknown>(`/jobs/${encodeURIComponent(id)}`));
}

/** Reads a finished job's result; a missing or malformed one counts as a failure. */
export function readGenerationResult(result: unknown): GenerationResult {
  const parsed = generationResultSchema.safeParse(result);
  return parsed.success ? parsed.data : { status: "failed", error: null, created: [], merged: [] };
}
