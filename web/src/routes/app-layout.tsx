import { data, Outlet } from "react-router";

import { AppShell } from "@/components/layout/app-shell";
import { VerifyEmailBanner } from "@/features/auth";
import { IdeaGenerationProvider } from "@/features/ideas";
import { syncEmailVerified } from "@/lib/.server/api.server";
import { requireSession } from "@/lib/.server/auth.server";
import { commitSession } from "@/lib/.server/session.server";

import type { Route } from "./+types/app-layout";

export async function loader({ request }: Route.LoaderArgs) {
  const active = await requireSession(request);
  let { session, setCookie } = active;
  // While unverified, ask the API: the link may have been used on another device.
  const synced = await syncEmailVerified(session).catch(() => null);
  if (synced) {
    session = synced;
    setCookie = commitSession(synced);
  }
  return data(
    { user: session.user },
    setCookie ? { headers: { "Set-Cookie": setCookie } } : undefined,
  );
}

export default function AppLayout({ loaderData }: Route.ComponentProps) {
  return (
    <AppShell user={loaderData.user}>
      <IdeaGenerationProvider>
        {/* Sessions sealed before this field existed lack it: treat them as verified. */}
        {loaderData.user.emailVerified === false && (
          <VerifyEmailBanner email={loaderData.user.email} />
        )}
        <Outlet />
      </IdeaGenerationProvider>
    </AppShell>
  );
}
