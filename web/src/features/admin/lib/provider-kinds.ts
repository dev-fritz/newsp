import type { ProviderKind } from "../types";

export const PROVIDER_KINDS = ["openai_compatible", "openai", "anthropic", "gemini"] as const;

/** Where a Docker Compose Ollama answers; the usual first provider. */
export const OLLAMA_BASE_URL = "http://ollama:11434/v1";

/** Base URLs the API falls back to when none is given (only for hosted providers). */
export const DEFAULT_BASE_URL: Record<ProviderKind, string> = {
  openai_compatible: OLLAMA_BASE_URL,
  openai: "https://api.openai.com/v1",
  anthropic: "https://api.anthropic.com",
  gemini: "https://generativelanguage.googleapis.com",
};

export const DEFAULT_PROVIDER_NAME: Record<ProviderKind, string> = {
  openai_compatible: "Ollama (local)",
  openai: "OpenAI",
  anthropic: "Anthropic",
  gemini: "Google Gemini",
};
