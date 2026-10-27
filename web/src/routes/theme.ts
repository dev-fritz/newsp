import { data } from "react-router";

import { preferenceCookie } from "@/lib/.server/cookies.server";
import { isTrustedMutation } from "@/lib/.server/security.server";
import { isTheme, THEME_COOKIE } from "@/lib/theme";

import type { Route } from "./+types/theme";

/** Stores the theme preference in a cookie so the server renders it without a flash. */
export async function action({ request }: Route.ActionArgs) {
  if (!isTrustedMutation(request)) throw data({ error: "forbidden" }, { status: 403 });
  const theme = (await request.formData()).get("theme");
  if (!isTheme(theme)) throw data({ error: "invalid theme" }, { status: 400 });
  return data({ ok: true }, { headers: { "Set-Cookie": preferenceCookie(THEME_COOKIE, theme) } });
}
