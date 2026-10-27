import { UserPlus } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { ErrorState, ListSkeleton } from "@/components/common/states";
import { PageHeader } from "@/components/common/page-header";
import { Button } from "@/components/ui/button";
import { useCan, useCurrentUser } from "@/features/auth";

import { useUsers } from "../api/users-queries";
import { errorOf, FORBIDDEN } from "../lib/error-of";
import { InviteDialog } from "./invite-dialog";
import { InvitesList } from "./invites-list";
import { UsersTable } from "./users-table";

/** /admin/users: people, their access, pending invites and one-time links. */
export function UsersPage() {
  const { t } = useTranslation("admin");
  const allowed = useCan("users:manage");
  const me = useCurrentUser();
  const users = useUsers();
  const [inviting, setInviting] = useState(false);

  const header = (
    <PageHeader
      kicker={t("kicker")}
      title={t("users.title")}
      description={t("users.description")}
      actions={
        allowed && (
          <Button onClick={() => setInviting(true)} className="h-9 pointer-coarse:h-11">
            <UserPlus aria-hidden />
            {t("users.invite.button")}
          </Button>
        )
      }
    />
  );

  if (!allowed) {
    return (
      <div className="flex flex-col gap-6">
        {header}
        <ErrorState error={FORBIDDEN} />
      </div>
    );
  }

  const list = users.data?.pages.flatMap((p) => p.data) ?? [];

  return (
    <div className="flex flex-col gap-7">
      {header}
      {users.isPending && <ListSkeleton rows={4} />}
      {users.isError && (
        <ErrorState error={errorOf(users.error)} onRetry={() => void users.refetch()} />
      )}
      {users.data && (
        <div className="flex flex-col gap-3">
          <UsersTable users={list} currentUserId={me.id} />
          {users.hasNextPage && (
            <Button
              variant="outline"
              className="self-center pointer-coarse:h-11"
              onClick={() => void users.fetchNextPage()}
              disabled={users.isFetchingNextPage}
            >
              {t("common:actions.loadMore")}
            </Button>
          )}
        </div>
      )}
      <InvitesList />
      <InviteDialog open={inviting} onOpenChange={setInviting} />
    </div>
  );
}
