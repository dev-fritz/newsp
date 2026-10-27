import { getPage, send } from "@/lib/http";
import type { PageMeta } from "@/lib/http";

import type { AdminUser, Invite, InviteLink, OneTimeLink, Role } from "../types";

export const USERS_PAGE_SIZE = 25;
const INVITES_LIMIT = 100;

export type UserAccessBody = { role?: Role; status?: "active" | "disabled" };

export function listUsers(offset: number): Promise<{ data: AdminUser[]; meta: PageMeta }> {
  return getPage<AdminUser>("/admin/users", { limit: USERS_PAGE_SIZE, offset });
}

export function updateUserAccess(id: string, body: UserAccessBody): Promise<AdminUser> {
  return send<AdminUser>("patch", `/admin/users/${encodeURIComponent(id)}`, body);
}

export function createResetLink(id: string): Promise<OneTimeLink> {
  return send<OneTimeLink>("post", `/admin/users/${encodeURIComponent(id)}/password-reset`);
}

export async function listInvites(): Promise<Invite[]> {
  const page = await getPage<Invite>("/admin/invites", { limit: INVITES_LIMIT, offset: 0 });
  return page.data;
}

export function createInvite(body: { role: Role; email?: string }): Promise<InviteLink> {
  return send<InviteLink>("post", "/admin/invites", body);
}

export function revokeInvite(id: string): Promise<void> {
  return send<void>("delete", `/admin/invites/${encodeURIComponent(id)}`);
}
