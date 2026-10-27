import {
  Cpu,
  FileText,
  Home,
  Layers,
  Lightbulb,
  LogOut,
  Menu,
  Moon,
  Rss,
  Sun,
  Users,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { Form, Link, NavLink, useFetcher, useRouteLoaderData } from "react-router";

import { BrandMark } from "@/components/common/brand-mark";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import type { SessionUser } from "@/lib/.server/session.server";
import { applyTheme, type Theme } from "@/lib/theme";
import { cn } from "@/lib/utils";

type NavItem = { to: string; label: string; icon: typeof Home; end?: boolean };

function useNav(user: SessionUser) {
  const { t } = useTranslation();
  const main: NavItem[] = [
    { to: "/", label: t("nav.home"), icon: Home, end: true },
    { to: "/reports", label: t("nav.reports"), icon: FileText },
    { to: "/ideas", label: t("nav.ideas"), icon: Lightbulb },
    { to: "/feed", label: t("nav.feed"), icon: Rss },
  ];
  const can = (p: string) => user.permissions.includes(p);
  const admin: NavItem[] = [
    can("sources:manage") && { to: "/admin/sources", label: t("nav.sources"), icon: Layers },
    can("providers:manage") && { to: "/admin/ai", label: t("nav.ai"), icon: Cpu },
    can("users:manage") && { to: "/admin/users", label: t("nav.users"), icon: Users },
  ].filter((x): x is NavItem => Boolean(x));
  return { main, admin };
}

function NavItems({ items, onNavigate }: { items: NavItem[]; onNavigate?: () => void }) {
  return (
    <div className="flex flex-col gap-0.5">
      {items.map(({ to, label, icon: Icon, end }) => (
        <NavLink
          key={to}
          to={to}
          end={end}
          onClick={onNavigate}
          className={({ isActive }) =>
            cn(
              "group flex min-h-10 items-center gap-2.5 rounded-lg px-2.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
              isActive && "bg-muted text-foreground [&>svg]:text-signal",
            )
          }
        >
          <Icon className="size-[18px]" aria-hidden />
          {label}
        </NavLink>
      ))}
    </div>
  );
}

function ThemeToggle() {
  const { t } = useTranslation();
  const fetcher = useFetcher();
  const root = useRouteLoaderData<{ theme: Theme }>("root");
  const toggle = () => {
    const next: Theme = document.documentElement.dataset.theme === "dark" ? "light" : "dark";
    applyTheme(next);
    void fetcher.submit({ theme: next }, { method: "post", action: "/theme" });
  };
  const dark = (fetcher.formData?.get("theme") ?? root?.theme) === "dark";
  return (
    <Button variant="ghost" size="icon" onClick={toggle} aria-label={t("nav.toggleTheme")}>
      {dark ? <Sun aria-hidden /> : <Moon aria-hidden />}
    </Button>
  );
}

function UserCard({ user }: { user: SessionUser }) {
  const { t } = useTranslation();
  const initials = user.name
    .split(/\s+/)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() ?? "")
    .join("");
  return (
    <div className="flex items-center gap-2 border-t border-border pt-3.5">
      <Link
        to="/profile"
        className="flex min-w-0 flex-1 items-center gap-2.5 rounded-lg px-1.5 py-1 hover:bg-muted"
      >
        <span
          aria-hidden
          className="inline-flex size-8 items-center justify-center rounded-full bg-foreground text-xs font-semibold text-background"
        >
          {initials}
        </span>
        <span className="flex min-w-0 flex-col leading-tight">
          <span className="truncate text-[13.5px] font-medium">{user.name}</span>
          <span className="text-xs text-subtle-foreground">{t(`roles.${user.role}`)}</span>
        </span>
      </Link>
      <ThemeToggle />
      <Form method="post" action="/logout">
        <Button type="submit" variant="ghost" size="icon" aria-label={t("nav.logout")}>
          <LogOut aria-hidden />
        </Button>
      </Form>
    </div>
  );
}

function Sidebar({ user, onNavigate }: { user: SessionUser; onNavigate?: () => void }) {
  const { t } = useTranslation();
  const { main, admin } = useNav(user);
  return (
    <div className="flex h-full flex-col gap-6 px-3.5 py-5">
      <Link to="/" onClick={onNavigate} className="px-2">
        <BrandMark className="text-[27px]" />
      </Link>
      <nav aria-label={t("nav.main")} className="flex flex-col gap-6">
        <NavItems items={main} onNavigate={onNavigate} />
        {admin.length > 0 && (
          <div className="flex flex-col gap-1">
            <span className="kicker px-2.5 pb-1.5">{t("nav.admin")}</span>
            <NavItems items={admin} onNavigate={onNavigate} />
          </div>
        )}
      </nav>
      <div className="mt-auto">
        <UserCard user={user} />
      </div>
    </div>
  );
}

/** Authenticated layout: sidebar on desktop; top bar, menu sheet and tabs on mobile. */
export function AppShell({ user, children }: { user: SessionUser; children: React.ReactNode }) {
  const { t } = useTranslation();
  const { main } = useNav(user);
  return (
    <div className="flex min-h-svh">
      <aside className="sticky top-0 hidden h-svh w-[248px] shrink-0 border-r border-border bg-card lg:block">
        <Sidebar user={user} />
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-20 flex items-center justify-between border-b border-border bg-card/95 px-2 py-1.5 backdrop-blur lg:hidden">
          <Sheet>
            <SheetTrigger asChild>
              <Button variant="ghost" size="icon" className="size-11" aria-label={t("nav.menu")}>
                <Menu aria-hidden />
              </Button>
            </SheetTrigger>
            <SheetContent side="left" className="w-[280px] p-0">
              <SheetTitle className="sr-only">{t("nav.menu")}</SheetTitle>
              <Sidebar user={user} />
            </SheetContent>
          </Sheet>
          <Link to="/">
            <BrandMark className="text-2xl" />
          </Link>
          <span className="size-11" aria-hidden />
        </header>

        <main className="flex-1 px-4 pt-6 pb-28 sm:px-8 lg:px-14 lg:pt-9 lg:pb-16 2xl:px-20">
          <div className="min-w-0">{children}</div>
        </main>

        <nav
          aria-label={t("nav.main")}
          className="fixed inset-x-0 bottom-0 z-20 flex border-t border-border bg-card px-2 pt-1.5 pb-[calc(6px+env(safe-area-inset-bottom))] lg:hidden"
        >
          {main.map(({ to, label, icon: Icon, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                cn(
                  "flex min-h-[52px] flex-1 flex-col items-center justify-center gap-1 text-[11px] font-medium text-subtle-foreground",
                  isActive && "text-foreground [&>svg]:text-signal",
                )
              }
            >
              <Icon className="size-[22px]" aria-hidden />
              {label}
            </NavLink>
          ))}
        </nav>
      </div>
    </div>
  );
}
