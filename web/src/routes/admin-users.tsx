import { UsersPage } from "@/features/admin";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/admin-users";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "users");
}

export default function Screen() {
  return <UsersPage />;
}
