import { z } from "zod";

import { audienceKindSchema, difficultySchema } from "./idea-response.schema";

// Messages are i18n keys under common:fieldErrors; limits mirror the API's validation.

export const IDEA_LIMITS = {
  title: 120,
  summary: 1000,
  longText: 8000,
  shortText: 4000,
  stackItems: 20,
  stackItemLength: 80,
  architectureItems: 20,
  architectureItemLength: 300,
  audienceMax: 8,
  planMin: 2,
  planMax: 12,
  competitorsMax: 12,
  effortMax: 104,
  focus: 200,
  instruction: 500,
  items: 5,
} as const;

/** One non-empty line per list item. */
export function splitLines(value: string): string[] {
  return value
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

function lines(maxItems: number, maxLength: number) {
  return z.string().superRefine((value, ctx) => {
    const items = splitLines(value);
    if (items.length > maxItems || items.some((item) => item.length > maxLength)) {
      ctx.addIssue({ code: "custom", message: "max" });
    }
  });
}

const required = (max: number) =>
  z.string().trim().min(1, { error: "required" }).max(max, { error: "max" });
const optional = (max: number) => z.string().trim().max(max, { error: "max" });

export const editIdeaSchema = z.object({
  title: required(IDEA_LIMITS.title),
  summary: required(IDEA_LIMITS.summary),
  difficulty: difficultySchema,
  effort_weeks: z
    .number({ error: "required" })
    .int({ error: "invalid" })
    .min(1, { error: "min" })
    .max(IDEA_LIMITS.effortMax, { error: "max" }),
  why_now: required(IDEA_LIMITS.longText),
  how: required(IDEA_LIMITS.longText),
  differentiator: optional(IDEA_LIMITS.shortText),
  monetization: optional(IDEA_LIMITS.shortText),
  stack: lines(IDEA_LIMITS.stackItems, IDEA_LIMITS.stackItemLength),
  architecture_steps: lines(IDEA_LIMITS.architectureItems, IDEA_LIMITS.architectureItemLength),
  audience: z
    .array(
      z.object({
        kind: audienceKindSchema,
        label: required(200),
        description: optional(1000),
      }),
    )
    .min(1, { error: "required" })
    .max(IDEA_LIMITS.audienceMax, { error: "max" }),
  mvp_plan: z
    .array(
      z.object({
        period: required(100),
        title: required(200),
        description: optional(1000),
      }),
    )
    .min(IDEA_LIMITS.planMin, { error: "min" })
    .max(IDEA_LIMITS.planMax, { error: "max" }),
  competitors: z
    .array(
      z.object({
        name: required(200),
        strengths: optional(1000),
        gap: optional(1000),
      }),
    )
    .max(IDEA_LIMITS.competitorsMax, { error: "max" }),
});
export type EditIdeaValues = z.infer<typeof editIdeaSchema>;

/** A feed item picked to seed a generation (kept whole so the dialog can show it). */
export const pickedItemSchema = z.object({
  id: z.string().min(1),
  title: z.string(),
  sourceKind: z.string().optional(),
  sourceName: z.string().optional(),
});

export const generateIdeasSchema = z.object({
  focus: z.string().trim().max(IDEA_LIMITS.focus, { error: "max" }),
  count: z.union([z.literal(1), z.literal(3), z.literal(5)]),
  window_days: z.union([z.literal(1), z.literal(7), z.literal(30)]),
  items: z.array(pickedItemSchema).max(IDEA_LIMITS.items, { error: "max" }),
});
export type GenerateIdeasValues = z.infer<typeof generateIdeasSchema>;

export const DEEPEN_ASPECTS = [
  "architecture",
  "mvp_plan",
  "market",
  "monetization",
  "audience",
  "competitors",
] as const;
export type DeepenAspect = (typeof DEEPEN_ASPECTS)[number];

export const deepenIdeaSchema = z.object({
  aspects: z.array(z.enum(DEEPEN_ASPECTS)),
  instruction: z.string().trim().max(IDEA_LIMITS.instruction, { error: "max" }),
});
export type DeepenIdeaValues = z.infer<typeof deepenIdeaSchema>;

export const IDEA_STATUS_FILTERS = ["active", "new", "saved", "discarded"] as const;
export type IdeaStatusFilter = (typeof IDEA_STATUS_FILTERS)[number];

/** Validated view of the list's search params; anything unknown falls back to the default. */
export const ideaListParamsSchema = z.object({
  status: z.enum(IDEA_STATUS_FILTERS).catch("active"),
  q: z.string().trim().max(100).catch(""),
});
export type IdeaListParams = z.infer<typeof ideaListParamsSchema>;

/** Compare page params: positive version numbers, or undefined for the defaults. */
const versionParam = z.coerce.number().int().min(1).optional().catch(undefined);
export const compareParamsSchema = z.object({ from: versionParam, to: versionParam });
