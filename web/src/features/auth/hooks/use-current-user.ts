import { useRouteLoaderData } from "react-router";

import type { SessionUser } from "@/lib/.server/session.server";

/** The signed-in user, available under the authenticated layout. */
export function useCurrentUser(): SessionUser {
  const data = useRouteLoaderData<{ user: SessionUser }>("routes/app-layout");
  if (!data) throw new Error("useCurrentUser must be used under the authenticated layout");
  return data.user;
}

/** Whether the user holds a permission ("resource:action"), as the API reports it. */
export function useCan(permission: string): boolean {
  return useCurrentUser().permissions.includes(permission);
}
