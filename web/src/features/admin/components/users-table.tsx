import { useTranslation } from "react-i18next";

import { Table, TableBody, TableHead, TableHeader, TableRow } from "@/components/ui/table";

import type { AdminUser } from "../types";
import { UserRow } from "./user-row";

type UsersTableProps = { users: AdminUser[]; currentUserId: string };

export function UsersTable({ users, currentUserId }: UsersTableProps) {
  const { t } = useTranslation("admin");
  return (
    <div className="overflow-hidden rounded-2xl border border-border bg-card">
      <Table className="min-w-[780px]">
        <TableHeader>
          <TableRow>
            <TableHead className="kicker">{t("users.table.person")}</TableHead>
            <TableHead className="kicker">{t("users.table.role")}</TableHead>
            <TableHead className="kicker">{t("users.table.status")}</TableHead>
            <TableHead className="kicker">{t("users.table.lastSeen")}</TableHead>
            <TableHead className="kicker text-right">{t("users.table.actions")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {users.map((u) => (
            <UserRow key={u.id} user={u} isSelf={u.id === currentUserId} />
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
