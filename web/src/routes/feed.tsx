import { FeedScreen } from "@/features/feed";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/feed";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "feed");
}

export default function Screen() {
  return <FeedScreen />;
}
