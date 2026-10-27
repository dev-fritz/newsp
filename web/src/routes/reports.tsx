import { Outlet, useParams } from "react-router";

import { ReportsLayout } from "@/features/reports";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/reports";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "reports");
}

// Layout: the report list plus the reader (`:id` child route). At /reports the
// reader shows the latest report, or an empty state.
export default function Reports() {
  const { id } = useParams();
  return (
    <ReportsLayout selectedId={id}>
      <Outlet />
    </ReportsLayout>
  );
}
