import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Outlet } from "react-router";

import { BrandMark } from "@/components/common/brand-mark";

/** Split layout of the sign-in screens: editorial panel and form. */
export default function AuthLayout() {
  const { t } = useTranslation("auth");
  const queryClient = useQueryClient();
  // Logout and expired sessions land here through client-side navigation, so
  // the root query cache would otherwise hand the previous account's data to
  // whoever signs in next in this tab.
  useEffect(() => {
    queryClient.clear();
  }, [queryClient]);
  return (
    <div className="flex min-h-svh flex-col lg:flex-row">
      <section className="flex flex-col justify-between gap-10 bg-[#0f1216] px-6 py-10 text-[#eceef1] sm:px-12 lg:w-[52%] lg:px-16 lg:py-12">
        <BrandMark className="text-3xl [&>span]:text-[#91a3ff]" />
        <div className="hidden max-w-xl flex-col gap-6 lg:flex">
          <h1 className="font-serif text-6xl leading-[1.02] font-medium tracking-tight text-balance">
            {t("brand.headline")}
          </h1>
          <p className="max-w-md text-lg leading-relaxed text-[#a3abb5]">{t("brand.subtitle")}</p>
        </div>
        <span className="hidden font-mono text-xs text-[#8d96a1] lg:block">
          {t("brand.footer")}
        </span>
      </section>
      <main className="flex flex-1 items-center justify-center bg-background px-6 py-12">
        <div className="w-full max-w-sm">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
