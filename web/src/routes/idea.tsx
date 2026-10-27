import { IdeaDetailView } from "@/features/ideas/components/idea-detail-view";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/idea";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "idea");
}

export default function IdeaRoute({ params }: Route.ComponentProps) {
  return <IdeaDetailView id={params.id} />;
}
