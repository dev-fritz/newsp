import { getData, getPage, send, type PageMeta } from "@/lib/http";

import type { Report, ReportCreated, ReportKind, ReportSummary } from "../types";

export type ReportsPage = { data: ReportSummary[]; meta: PageMeta };

export function listReports(params: {
  kind?: ReportKind;
  limit: number;
  offset: number;
}): Promise<ReportsPage> {
  return getPage<ReportSummary>("/reports", params);
}

export function getReport(id: string): Promise<Report> {
  return getData<Report>(`/reports/${encodeURIComponent(id)}`);
}

export type CreateReportInput = { window_hours: number; focus?: string };

export function createReport(input: CreateReportInput): Promise<ReportCreated> {
  return send<ReportCreated>("post", "/reports", input);
}

export function deleteReport(id: string): Promise<void> {
  return send<void>("delete", `/reports/${encodeURIComponent(id)}`);
}
