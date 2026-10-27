import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  collectSource,
  createSource,
  deleteSource,
  listSources,
  updateSource,
  type CreateSourceBody,
  type UpdateSourceBody,
} from "./sources-api";

export const sourceKeys = {
  all: ["admin", "sources"] as const,
  list: () => [...sourceKeys.all, "list"] as const,
};

export function useSources() {
  return useQuery({ queryKey: sourceKeys.list(), queryFn: listSources });
}

export function useCreateSource() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateSourceBody) => createSource(body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: sourceKeys.all }),
  });
}

export function useUpdateSource() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (v: { id: string; body: UpdateSourceBody }) => updateSource(v.id, v.body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: sourceKeys.all }),
  });
}

export function useDeleteSource() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteSource(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: sourceKeys.all }),
  });
}

export function useCollectSource() {
  return useMutation({ mutationFn: (id: string) => collectSource(id) });
}
