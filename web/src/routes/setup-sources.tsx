import { SetupSourcesPage } from "@/features/admin";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/setup-sources";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "setupSources");
}

export default function Screen() {
  return <SetupSourcesPage />;
}
