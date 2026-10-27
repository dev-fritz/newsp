import { diffPaired, diffWords, hasChanges, type DiffOp } from "./diff";
import type { Difficulty, Evidence, IdeaVersion } from "../schemas/idea-response.schema";

export type RowStatus = "equal" | "change" | "add" | "remove";
export type DiffField = { name: string; ops: DiffOp[] };
export type DiffRow = { status: RowStatus; fields: DiffField[] };

export type SectionId =
  | "title"
  | "summary"
  | "meta"
  | "why_now"
  | "audience"
  | "how"
  | "differentiator"
  | "stack"
  | "architecture_steps"
  | "mvp_plan"
  | "monetization"
  | "competitors"
  | "evidence";

export type SectionBody =
  | { type: "text"; ops: DiffOp[] }
  | { type: "rows"; rows: DiffRow[] }
  | {
      type: "meta";
      difficulty: { before: Difficulty; after: Difficulty };
      effort: { before: number | null; after: number | null };
    }
  | { type: "evidence"; added: Evidence[]; removed: Evidence[]; kept: Evidence[] };

export type SectionDiff = {
  id: SectionId;
  /** Whether anything differs between the two versions. */
  changed: boolean;
  /** Nothing to show on either side (an optional section left blank). */
  empty: boolean;
  body: SectionBody;
};

export type VersionDiff = {
  sections: SectionDiff[];
  changedCount: number;
  unchangedCount: number;
  evidenceAdded: number;
  evidenceRemoved: number;
};

function textSection(id: SectionId, before: string, after: string): SectionDiff {
  const ops = diffWords(before, after);
  return {
    id,
    changed: hasChanges(ops),
    empty: before.trim() === "" && after.trim() === "",
    body: { type: "text", ops },
  };
}

type Fields = { [field: string]: string };

/** Rows compared item by item; an item edited in place becomes one changed row. */
function rowsSection<T extends Fields>(
  id: SectionId,
  before: readonly T[],
  after: readonly T[],
  fields: readonly (keyof T & string)[],
): SectionDiff {
  const keyOf = (item: T) => fields.map((f) => item[f] ?? "").join("\u0000");
  const rows: DiffRow[] = diffPaired(before, after, keyOf).map((op): DiffRow => {
    switch (op.type) {
      case "change":
        return {
          status: "change",
          fields: fields.map((f) => ({
            name: f,
            ops: diffWords(op.before[f] ?? "", op.after[f] ?? ""),
          })),
        };
      case "equal":
        return {
          status: "equal",
          fields: fields.map((f) => ({
            name: f,
            ops: [{ type: "equal", text: op.item[f] ?? "" }],
          })),
        };
      case "add":
        return {
          status: "add",
          fields: fields.map((f) => ({ name: f, ops: [{ type: "add", text: op.item[f] ?? "" }] })),
        };
      case "remove":
        return {
          status: "remove",
          fields: fields.map((f) => ({
            name: f,
            ops: [{ type: "remove", text: op.item[f] ?? "" }],
          })),
        };
    }
  });
  return {
    id,
    changed: rows.some((row) => row.status !== "equal"),
    empty: before.length === 0 && after.length === 0,
    body: { type: "rows", rows },
  };
}

function linesSection(id: SectionId, before: readonly string[], after: readonly string[]) {
  return rowsSection(
    id,
    before.map((value) => ({ value })),
    after.map((value) => ({ value })),
    ["value"],
  );
}

function evidenceSection(before: readonly Evidence[], after: readonly Evidence[]): SectionDiff {
  const beforeIds = new Set(before.map((e) => e.item_id));
  const afterIds = new Set(after.map((e) => e.item_id));
  const added = after.filter((e) => !beforeIds.has(e.item_id));
  const removed = before.filter((e) => !afterIds.has(e.item_id));
  const kept = after.filter((e) => beforeIds.has(e.item_id));
  return {
    id: "evidence",
    changed: added.length > 0 || removed.length > 0,
    empty: before.length === 0 && after.length === 0,
    body: { type: "evidence", added, removed, kept },
  };
}

/** Everything that differs between two versions of an idea, section by section. */
export function diffVersions(from: IdeaVersion, to: IdeaVersion): VersionDiff {
  const a = from.content;
  const b = to.content;
  const meta: SectionDiff = {
    id: "meta",
    changed: from.difficulty !== to.difficulty || from.effort_weeks !== to.effort_weeks,
    empty: false,
    body: {
      type: "meta",
      difficulty: { before: from.difficulty, after: to.difficulty },
      effort: { before: from.effort_weeks, after: to.effort_weeks },
    },
  };
  const all: SectionDiff[] = [
    textSection("title", from.title, to.title),
    textSection("summary", from.summary, to.summary),
    meta,
    textSection("why_now", a.why_now, b.why_now),
    rowsSection("audience", a.audience, b.audience, ["kind", "label", "description"]),
    textSection("how", a.how, b.how),
    textSection("differentiator", a.differentiator, b.differentiator),
    linesSection("stack", a.stack, b.stack),
    linesSection("architecture_steps", a.architecture_steps, b.architecture_steps),
    rowsSection("mvp_plan", a.mvp_plan, b.mvp_plan, ["period", "title", "description"]),
    textSection("monetization", a.monetization, b.monetization),
    rowsSection("competitors", a.competitors, b.competitors, ["name", "strengths", "gap"]),
    evidenceSection(from.evidence, to.evidence),
  ];
  const sections = all.filter((section) => !section.empty);
  const evidence = sections.find((s) => s.id === "evidence");
  const evidenceBody = evidence?.body.type === "evidence" ? evidence.body : undefined;
  return {
    sections,
    changedCount: sections.filter((s) => s.changed).length,
    unchangedCount: sections.filter((s) => !s.changed).length,
    evidenceAdded: evidenceBody?.added.length ?? 0,
    evidenceRemoved: evidenceBody?.removed.length ?? 0,
  };
}
