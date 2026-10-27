import { QueryClientProvider } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { I18nextProvider } from "react-i18next";
import {
  isRouteErrorResponse,
  Links,
  Meta,
  Outlet,
  Scripts,
  ScrollRestoration,
  useRouteLoaderData,
} from "react-router";

import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { createI18n, isLocale, localeFromHeader, type Locale } from "@/i18n";
import { readCookie } from "@/lib/.server/cookies.server";
import { getSession } from "@/lib/.server/session.server";
import { useNonce } from "@/lib/nonce";
import { createQueryClient } from "@/lib/query-client";
import { isTheme, systemThemeScript, THEME_COOKIE, type Theme } from "@/lib/theme";

import type { Route } from "./+types/root";
import "@/styles/index.css";

export async function loader({ request }: Route.LoaderArgs) {
  const session = getSession(request);
  const cookieLocale = readCookie(request, "newsp_lang");
  const locale: Locale =
    session?.user.locale ??
    (isLocale(cookieLocale)
      ? cookieLocale
      : localeFromHeader(request.headers.get("Accept-Language")));
  const themeCookie = readCookie(request, THEME_COOKIE);
  const theme: Theme = isTheme(themeCookie) ? themeCookie : "system";
  return { locale, theme };
}

export const links: Route.LinksFunction = () => [{ rel: "icon", href: "/favicon.ico" }];

export function Layout({ children }: { children: React.ReactNode }) {
  const data = useRouteLoaderData<typeof loader>("root");
  const nonce = useNonce();
  const theme = data?.theme ?? "system";
  return (
    <html
      lang={data?.locale ?? "pt-BR"}
      data-theme={theme === "dark" ? "dark" : "light"}
      suppressHydrationWarning
    >
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover" />
        {theme === "system" && (
          <script nonce={nonce} dangerouslySetInnerHTML={{ __html: systemThemeScript }} />
        )}
        <Meta />
        <Links />
      </head>
      <body>
        {children}
        <ScrollRestoration nonce={nonce} />
        <Scripts nonce={nonce} />
      </body>
    </html>
  );
}

export default function App({ loaderData }: Route.ComponentProps) {
  const [queryClient] = useState(createQueryClient);
  const i18n = useMemo(() => createI18n(loaderData.locale), [loaderData.locale]);
  return (
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <TooltipProvider delayDuration={300}>
          <Outlet />
          <Toaster theme={loaderData.theme} position="bottom-right" />
        </TooltipProvider>
      </QueryClientProvider>
    </I18nextProvider>
  );
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  const notFound = isRouteErrorResponse(error) && error.status === 404;
  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col justify-center gap-3 p-6">
      <span className="font-serif text-3xl font-semibold tracking-tight">
        newsp<span className="text-signal">.</span>
      </span>
      <h1 className="font-serif text-4xl font-medium">{notFound ? "404" : "Erro / Error"}</h1>
      <p className="text-muted-foreground">
        {notFound
          ? "Página não encontrada. / Page not found."
          : "Algo deu errado. / Something went wrong."}
      </p>
      <a className="text-signal underline-offset-4 hover:underline" href="/">
        Início / Home
      </a>
    </main>
  );
}
