import { useSearchParams } from "react-router";

import { IdeaCompareView } from "@/features/ideas/components/idea-compare-view";
import { compareParamsSchema } from "@/features/ideas/schemas/idea.schema";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/idea-compare";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "ideaCompare");
}

export default function IdeaCompareRoute({ params }: Route.ComponentProps) {
  const [search, setSearch] = useSearchParams();
  const { from, to } = compareParamsSchema.parse({
    from: search.get("from") ?? undefined,
    to: search.get("to") ?? undefined,
  });
  return (
    <IdeaCompareView
      id={params.id}
      from={from}
      to={to}
      onVersionsChange={(versions) =>
        setSearch({ from: String(versions.from), to: String(versions.to) }, { replace: true })
      }
    />
  );
}
