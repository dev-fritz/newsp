import { z } from "zod";

// The API's OpenAPI types mark every field optional and Go serializes nil slices
// as null, so responses are parsed once at the boundary into complete shapes.
const text = z
  .string()
  .nullish()
  .transform((v) => v ?? "");
const integer = z
  .number()
  .nullish()
  .transform((v) => v ?? 0);
const optionalNumber = z
  .number()
  .nullish()
  .transform((v) => v ?? null);
const optionalText = z
  .string()
  .nullish()
  .transform((v) => v ?? null);
function list<T extends z.ZodType>(item: T) {
  return z
    .array(item)
    .nullish()
    .transform((v) => v ?? []);
}

export const ideaStatusSchema = z.enum(["new", "saved", "discarded"]);
export type IdeaStatus = z.infer<typeof ideaStatusSchema>;

export const versionOriginSchema = z.enum(["generated", "merged", "deepened", "manual"]);
export type VersionOrigin = z.infer<typeof versionOriginSchema>;

export const difficultySchema = z.enum(["beginner", "intermediate", "advanced"]);
export type Difficulty = z.infer<typeof difficultySchema>;

export const audienceKindSchema = z.enum(["primary", "secondary", "out_of_scope"]);
export type AudienceKind = z.infer<typeof audienceKindSchema>;

const audienceSegmentSchema = z.object({
  kind: audienceKindSchema.catch("secondary"),
  label: text,
  description: text,
});
const planStepSchema = z.object({ period: text, title: text, description: text });
const competitorSchema = z.object({ name: text, strengths: text, gap: text });

export const ideaContentSchema = z.object({
  why_now: text,
  audience: list(audienceSegmentSchema),
  how: text,
  differentiator: text,
  stack: list(z.string()),
  architecture_steps: list(z.string()),
  mvp_plan: list(planStepSchema),
  monetization: text,
  competitors: list(competitorSchema),
});
export type IdeaContent = z.infer<typeof ideaContentSchema>;
export type AudienceSegment = z.infer<typeof audienceSegmentSchema>;
export type PlanStep = z.infer<typeof planStepSchema>;
export type Competitor = z.infer<typeof competitorSchema>;

export const evidenceSchema = z.object({
  ref: text,
  item_id: text,
  url: text,
  title: text,
  source_kind: text,
  source_name: text,
  published_at: text,
  score: optionalNumber,
  comments: optionalNumber,
  excerpt: text,
});
export type Evidence = z.infer<typeof evidenceSchema>;

export const ideaVersionSchema = z.object({
  version: integer,
  origin: versionOriginSchema.catch("generated"),
  title: text,
  summary: text,
  difficulty: difficultySchema.catch("intermediate"),
  effort_weeks: optionalNumber,
  content: ideaContentSchema,
  change_note: optionalText,
  similarity: optionalNumber,
  model_name: optionalText,
  created_at: text,
  evidence: list(evidenceSchema),
});
export type IdeaVersion = z.infer<typeof ideaVersionSchema>;

const versionSummarySchema = z.object({
  version: integer,
  origin: versionOriginSchema.catch("generated"),
  change_note: optionalText,
  similarity: optionalNumber,
  model_name: optionalText,
  created_at: text,
});
export type IdeaVersionSummary = z.infer<typeof versionSummarySchema>;

export const ideaSchema = z.object({
  id: text,
  status: ideaStatusSchema.catch("new"),
  current_version: integer,
  merged_candidates: integer,
  created_at: text,
  updated_at: text,
  current: ideaVersionSchema,
  versions: list(versionSummarySchema),
});
export type Idea = z.infer<typeof ideaSchema>;

export const ideaListItemSchema = z.object({
  id: text,
  status: ideaStatusSchema.catch("new"),
  title: text,
  summary: text,
  current_version: integer,
  last_origin: versionOriginSchema.catch("generated"),
  difficulty: difficultySchema.catch("intermediate"),
  effort_weeks: optionalNumber,
  evidence_count: integer,
  primary_audience: text,
  merged_candidates: integer,
  created_at: text,
  updated_at: text,
});
export type IdeaListItem = z.infer<typeof ideaListItemSchema>;

/** A generation or deepen job's `result` once the job itself is done. */
export const generationResultSchema = z.object({
  status: z.enum(["done", "failed"]).catch("failed"),
  error: optionalText,
  created: list(z.string()),
  merged: list(z.string()),
});
export type GenerationResult = z.infer<typeof generationResultSchema>;
