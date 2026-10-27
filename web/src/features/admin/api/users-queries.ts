import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createInvite,
  createResetLink,
  listInvites,
  listUsers,
  revokeInvite,
  updateUserAccess,
  type UserAccessBody,
} from "./users-api";
import type { Role } from "../types";

export const userKeys = {
  all: ["admin", "users"] as const,
  list: () => [...userKeys.all, "list"] as const,
};

export const inviteKeys = {
  all: ["admin", "invites"] as const,
  list: () => [...inviteKeys.all, "list"] as const,
};

export function useUsers() {
  return useInfiniteQuery({
    queryKey: userKeys.list(),
    queryFn: ({ pageParam }) => listUsers(pageParam),
    initialPageParam: 0,
    getNextPageParam: (last) => {
      const next = last.meta.offset + last.meta.limit;
      return next < last.meta.total ? next : undefined;
    },
  });
}

export function useUpdateUserAccess() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (v: { id: string; body: UserAccessBody }) => updateUserAccess(v.id, v.body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: userKeys.all }),
  });
}

export function useCreateResetLink() {
  return useMutation({ mutationFn: (id: string) => createResetLink(id) });
}

export function useInvites() {
  return useQuery({ queryKey: inviteKeys.list(), queryFn: listInvites });
}

export function useCreateInvite() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: { role: Role; email?: string }) => createInvite(body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: inviteKeys.all }),
  });
}

export function useRevokeInvite() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => revokeInvite(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: inviteKeys.all }),
  });
}
