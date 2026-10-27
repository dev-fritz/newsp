import type { EditIdeaRequest } from "../api/ideas-api";
import type { IdeaVersion } from "../schemas/idea-response.schema";
import { splitLines, type EditIdeaValues } from "../schemas/idea.schema";

/** The edit form's starting values: the current version, with list fields as one item per line. */
export function toFormValues(version: IdeaVersion): EditIdeaValues {
  const { content } = version;
  return {
    title: version.title,
    summary: version.summary,
    difficulty: version.difficulty,
    effort_weeks: version.effort_weeks ?? 1,
    why_now: content.why_now,
    how: content.how,
    differentiator: content.differentiator,
    monetization: content.monetization,
    stack: content.stack.join("\n"),
    architecture_steps: content.architecture_steps.join("\n"),
    audience: content.audience.map((a) => ({ ...a })),
    mvp_plan: content.mvp_plan.map((s) => ({ ...s })),
    competitors: content.competitors.map((c) => ({ ...c })),
  };
}

/** The PUT body for validated form values. */
export function toEditRequest(values: EditIdeaValues): EditIdeaRequest {
  return {
    title: values.title,
    summary: values.summary,
    difficulty: values.difficulty,
    effort_weeks: values.effort_weeks,
    content: {
      why_now: values.why_now,
      how: values.how,
      differentiator: values.differentiator,
      monetization: values.monetization,
      stack: splitLines(values.stack),
      architecture_steps: splitLines(values.architecture_steps),
      audience: values.audience,
      mvp_plan: values.mvp_plan,
      competitors: values.competitors,
    },
  };
}
