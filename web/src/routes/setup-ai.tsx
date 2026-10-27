import { SetupAiPage } from "@/features/admin";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/setup-ai";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "setupAi");
}

export default function Screen() {
  return <SetupAiPage />;
}
