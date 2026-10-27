import { useTranslation } from "react-i18next";
import { data, Link } from "react-router";

import { BrandMark } from "@/components/common/brand-mark";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/not-found";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "notFound");
}

export function loader() {
  throw data(null, { status: 404 });
}

export function ErrorBoundary() {
  const { t } = useTranslation();
  return (
    <main className="mx-auto flex min-h-svh max-w-xl flex-col justify-center gap-4 p-6">
      <BrandMark className="text-3xl" />
      <h1 className="font-serif text-4xl font-medium">{t("states.notFoundTitle")}</h1>
      <p className="text-muted-foreground">{t("states.notFoundBody")}</p>
      <Link to="/" className="text-signal">
        {t("states.goHome")}
      </Link>
    </main>
  );
}

export default function NotFound() {
  return null;
}
