import { type RouteConfig, index, layout, route } from "@react-router/dev/routes";

export default [
  // Same-origin proxy to the API and small server actions.
  route("api/*", "routes/api.ts"),
  route("theme", "routes/theme.ts"),
  route("logout", "routes/logout.ts"),

  layout("routes/auth-layout.tsx", [
    route("login", "routes/login.tsx"),
    route("signup", "routes/signup.tsx"),
    route("setup", "routes/setup.tsx"),
    route("invite/:token", "routes/invite.tsx"),
    route("forgot-password", "routes/forgot-password.tsx"),
    route("reset-password/:token", "routes/reset-password.tsx"),
    route("verify-email/:token", "routes/verify-email.tsx"),
  ]),

  layout("routes/app-layout.tsx", [
    index("routes/home.tsx"),
    route("setup/ai", "routes/setup-ai.tsx"),
    route("setup/sources", "routes/setup-sources.tsx"),
    route("reports", "routes/reports.tsx", [route(":id", "routes/report.tsx")]),
    route("ideas", "routes/ideas.tsx"),
    route("ideas/:id", "routes/idea.tsx"),
    route("ideas/:id/compare", "routes/idea-compare.tsx"),
    route("feed", "routes/feed.tsx"),
    route("profile", "routes/profile.tsx"),
    route("admin/sources", "routes/admin-sources.tsx"),
    route("admin/ai", "routes/admin-ai.tsx"),
    route("admin/users", "routes/admin-users.tsx"),
  ]),

  route("*", "routes/not-found.tsx"),
] satisfies RouteConfig;
