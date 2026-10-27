import { HomeScreen } from "@/features/home";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/home";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "home");
}

export default function Screen() {
  return <HomeScreen />;
}
