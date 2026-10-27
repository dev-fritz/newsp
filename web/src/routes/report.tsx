import { useParams } from "react-router";

import { ReportReader } from "@/features/reports";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/report";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "report");
}

// /reports/:id, rendered inside the reports layout.
export default function Report() {
  const { id } = useParams();
  if (!id) return null;
  return <ReportReader id={id} />;
}
