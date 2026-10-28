import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createOwnModel,
  createOwnProvider,
  deleteOwnModel,
  deleteOwnProvider,
  fetchChatModels,
  fetchOwnProviders,
  fetchPreferences,
  fetchSources,
  savePreferences,
  testOwnProvider,
  updateOwnModel,
  updateOwnProvider,
  type ChatModel,
  type ConnectionTest,
  type CreateOwnModelRequest,
  type CreateOwnProviderRequest,
  type OwnModel,
  type OwnProvider,
  type OwnProviders,
  type Preferences,
  type PreferencesRequest,
  type SourceSummary,
  type UpdateOwnModelRequest,
  type UpdateOwnProviderRequest,
} from "@/features/profile/api/profile-api";
import type { ApiError } from "@/lib/api-error";

export const profileKeys = {
  all: ["profile"] as const,
  preferences: () => [...profileKeys.all, "preferences"] as const,
  models: () => [...profileKeys.all, "models"] as const,
  sources: () => [...profileKeys.all, "sources"] as const,
  ownProviders: () => [...profileKeys.all, "own-providers"] as const,
};

export function usePreferences() {
  return useQuery<Preferences, ApiError>({
    queryKey: profileKeys.preferences(),
    queryFn: fetchPreferences,
  });
}

export function useChatModels() {
  return useQuery<ChatModel[], ApiError>({
    queryKey: profileKeys.models(),
    queryFn: fetchChatModels,
  });
}

export function useSourceOptions() {
  return useQuery<SourceSummary[], ApiError>({
    queryKey: profileKeys.sources(),
    queryFn: fetchSources,
  });
}

export function useSavePreferences() {
  const client = useQueryClient();
  return useMutation<Preferences, ApiError, PreferencesRequest>({
    mutationFn: savePreferences,
    onSuccess: (saved) => {
      client.setQueryData(profileKeys.preferences(), saved);
      return client.invalidateQueries({ queryKey: profileKeys.preferences() });
    },
  });
}

export function useOwnProviders() {
  return useQuery<OwnProviders, ApiError>({
    queryKey: profileKeys.ownProviders(),
    queryFn: fetchOwnProviders,
  });
}

/** Own providers feed the model pickers too, so both lists are refreshed. */
function useRefreshOwn() {
  const client = useQueryClient();
  return () =>
    Promise.all([
      client.invalidateQueries({ queryKey: profileKeys.ownProviders() }),
      client.invalidateQueries({ queryKey: profileKeys.models() }),
    ]);
}

export function useCreateOwnProvider() {
  const refresh = useRefreshOwn();
  return useMutation<OwnProvider, ApiError, CreateOwnProviderRequest>({
    mutationFn: createOwnProvider,
    // PROVIDER_KIND_TAKEN means another tab connected it: refresh either way.
    onSettled: refresh,
  });
}

export function useUpdateOwnProvider() {
  const refresh = useRefreshOwn();
  return useMutation<OwnProvider, ApiError, { id: string; body: UpdateOwnProviderRequest }>({
    mutationFn: (v) => updateOwnProvider(v.id, v.body),
    onSuccess: refresh,
  });
}

export function useDeleteOwnProvider() {
  const client = useQueryClient();
  const refresh = useRefreshOwn();
  return useMutation<void, ApiError, string>({
    mutationFn: deleteOwnProvider,
    // Preferences pointing at a deleted model fall back to the instance default.
    onSuccess: () =>
      Promise.all([refresh(), client.invalidateQueries({ queryKey: profileKeys.preferences() })]),
  });
}

/** A failed test is a 200 with ok=false; either way the provider's check status changed. */
export function useTestOwnProvider() {
  const client = useQueryClient();
  return useMutation<ConnectionTest, ApiError, string>({
    mutationFn: testOwnProvider,
    onSettled: () => client.invalidateQueries({ queryKey: profileKeys.ownProviders() }),
  });
}

export function useCreateOwnModel() {
  const refresh = useRefreshOwn();
  return useMutation<OwnModel, ApiError, { providerId: string; body: CreateOwnModelRequest }>({
    mutationFn: (v) => createOwnModel(v.providerId, v.body),
    onSuccess: refresh,
  });
}

export function useUpdateOwnModel() {
  const refresh = useRefreshOwn();
  return useMutation<OwnModel, ApiError, { id: string; body: UpdateOwnModelRequest }>({
    mutationFn: (v) => updateOwnModel(v.id, v.body),
    onSuccess: refresh,
  });
}

export function useDeleteOwnModel() {
  const client = useQueryClient();
  const refresh = useRefreshOwn();
  return useMutation<void, ApiError, string>({
    mutationFn: deleteOwnModel,
    onSuccess: () =>
      Promise.all([refresh(), client.invalidateQueries({ queryKey: profileKeys.preferences() })]),
  });
}
