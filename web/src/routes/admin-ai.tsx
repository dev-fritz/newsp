import { AiProvidersPage } from "@/features/admin";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/admin-ai";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "ai");
}

export default function Screen() {
  return <AiProvidersPage />;
}
