import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createModel,
  createProvider,
  deleteModel,
  deleteProvider,
  getSettings,
  listProviders,
  testProvider,
  updateModel,
  updateProvider,
  updateSettings,
  type CreateModelBody,
  type CreateProviderBody,
  type UpdateModelBody,
  type UpdateProviderBody,
  type UpdateSettingsBody,
} from "./ai-api";

export const providerKeys = {
  all: ["admin", "ai", "providers"] as const,
  list: () => [...providerKeys.all, "list"] as const,
};

export const settingsKeys = {
  all: ["admin", "ai", "settings"] as const,
};

export function useProviders() {
  return useQuery({ queryKey: providerKeys.list(), queryFn: listProviders });
}

export function useSettings(enabled = true) {
  return useQuery({ queryKey: settingsKeys.all, queryFn: getSettings, enabled });
}

export function useCreateProvider() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateProviderBody) => createProvider(body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: providerKeys.all }),
  });
}

export function useUpdateProvider() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (v: { id: string; body: UpdateProviderBody }) => updateProvider(v.id, v.body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: providerKeys.all }),
  });
}

export function useDeleteProvider() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteProvider(id),
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: providerKeys.all }),
        // Deleting a provider can clear the models picked in the settings.
        queryClient.invalidateQueries({ queryKey: settingsKeys.all }),
      ]),
  });
}

/** A failed test is a 200 with ok=false; either way the provider's check status changed. */
export function useTestProvider() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => testProvider(id),
    onSettled: () => queryClient.invalidateQueries({ queryKey: providerKeys.all }),
  });
}

export function useCreateModel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (v: { providerId: string; body: CreateModelBody }) =>
      createModel(v.providerId, v.body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: providerKeys.all }),
  });
}

export function useUpdateModel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (v: { id: string; body: UpdateModelBody }) => updateModel(v.id, v.body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: providerKeys.all }),
  });
}

export function useDeleteModel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteModel(id),
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: providerKeys.all }),
        queryClient.invalidateQueries({ queryKey: settingsKeys.all }),
      ]),
  });
}

export function useUpdateSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateSettingsBody) => updateSettings(body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: settingsKeys.all }),
  });
}
