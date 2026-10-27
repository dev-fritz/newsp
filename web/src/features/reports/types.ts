import type { components } from "@/types/api";

type Schemas = components["schemas"];

export type ReportSummary = Schemas["dtos.ReportSummaryResponse"];
export type Report = Schemas["dtos.ReportResponse"];
export type Citation = Schemas["dtos.CitationResponse"];
export type ReportCreated = Schemas["dtos.ReportCreatedResponse"];

export type ReportKind = "scheduled" | "manual";
export type ReportStatus = "pending" | "running" | "done" | "failed";

/** Window choices offered by the "generate now" form, in hours. */
export const WINDOW_CHOICES = [6, 24, 168, 720] as const;
export type WindowChoice = (typeof WINDOW_CHOICES)[number];

export function isReportKind(value: unknown): value is ReportKind {
  return value === "scheduled" || value === "manual";
}

/** A report that is still being generated (the UI polls while any exists). */
export function isActive(report: { status?: string } | null | undefined): boolean {
  return report?.status === "pending" || report?.status === "running";
}
