import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  fetchChatModels,
  fetchPreferences,
  fetchSources,
  savePreferences,
  type ChatModel,
  type Preferences,
  type PreferencesRequest,
  type SourceSummary,
} from "@/features/profile/api/profile-api";
import type { ApiError } from "@/lib/api-error";

export const profileKeys = {
  all: ["profile"] as const,
  preferences: () => [...profileKeys.all, "preferences"] as const,
  models: () => [...profileKeys.all, "models"] as const,
  sources: () => [...profileKeys.all, "sources"] as const,
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
