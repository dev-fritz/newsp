import { SourcesPage } from "@/features/admin";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/admin-sources";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "sources");
}

export default function Screen() {
  return <SourcesPage />;
}
