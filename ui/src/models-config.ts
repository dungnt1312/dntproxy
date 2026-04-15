export interface ModelConfig {
  id: string;
  name: string;
  provider:
    | "kiro"
    | "openai"
    | "openai-compatible"
    | "anthropic"
    | "deepseek"
    | "google"
    | "other";
  prefix: string;
}

export const PROVIDER_CONFIGS: Record<
  string,
  { name: string; icon: string; color: string }
> = {
  kiro: { name: "Kiro", icon: "KI", color: "text-green-400" },
  openai: { name: "OpenAI", icon: "OA", color: "text-emerald-400" },
  "openai-compatible": {
    name: "OpenAI Compatible",
    icon: "OC",
    color: "text-blue-400",
  },
  anthropic: { name: "Anthropic", icon: "AN", color: "text-orange-400" },
  deepseek: { name: "DeepSeek", icon: "DS", color: "text-cyan-400" },
  google: { name: "Google", icon: "GO", color: "text-yellow-400" },
  other: { name: "Other", icon: "OT", color: "text-gray-400" },
};

export const MODELS_CONFIG: ModelConfig[] = [
  // Kiro models
  {
    id: "kiro/claude-sonnet-4.5",
    name: "Claude Sonnet 4.5",
    provider: "kiro",
    prefix: "kiro/",
  },
  {
    id: "kiro/claude-haiku-4.5",
    name: "Claude Haiku 4.5",
    provider: "kiro",
    prefix: "kiro/",
  },
  {
    id: "kiro/deepseek-3.2",
    name: "DeepSeek 3.2",
    provider: "kiro",
    prefix: "kiro/",
  },
  {
    id: "kiro/qwen3-coder-next",
    name: "Qwen3 Coder Next",
    provider: "kiro",
    prefix: "kiro/",
  },

  // OpenAI models
  {
    id: "openai/gpt-4.1",
    name: "GPT-4.1",
    provider: "openai",
    prefix: "openai/",
  },
  {
    id: "openai/gpt-4.1-mini",
    name: "GPT-4.1 Mini",
    provider: "openai",
    prefix: "openai/",
  },
  {
    id: "openai/gpt-4.1-nano",
    name: "GPT-4.1 Nano",
    provider: "openai",
    prefix: "openai/",
  },
  {
    id: "openai/gpt-4o",
    name: "GPT-4o",
    provider: "openai",
    prefix: "openai/",
  },
  {
    id: "openai/gpt-4o-mini",
    name: "GPT-4o Mini",
    provider: "openai",
    prefix: "openai/",
  },
  { id: "openai/o3", name: "o3", provider: "openai", prefix: "openai/" },
  {
    id: "openai/o3-mini",
    name: "o3 Mini",
    provider: "openai",
    prefix: "openai/",
  },
  {
    id: "openai/o4-mini",
    name: "o4 Mini",
    provider: "openai",
    prefix: "openai/",
  },

  // Anthropic models
  {
    id: "anthropic/claude-sonnet-4-20250514",
    name: "Claude Sonnet 4 (May)",
    provider: "anthropic",
    prefix: "anthropic/",
  },
  {
    id: "anthropic/claude-haiku-4-20251001",
    name: "Claude Haiku 4 (Oct)",
    provider: "anthropic",
    prefix: "anthropic/",
  },
  {
    id: "anthropic/claude-opus-4-5-20250514",
    name: "Claude Opus 4.5 (May)",
    provider: "anthropic",
    prefix: "anthropic/",
  },

  // DeepSeek models
  {
    id: "deepseek/deepseek-chat-v3-0324",
    name: "DeepSeek Chat V3",
    provider: "deepseek",
    prefix: "deepseek/",
  },
  {
    id: "deepseek/deepseek-coder-v3-0324",
    name: "DeepSeek Coder V3",
    provider: "deepseek",
    prefix: "deepseek/",
  },
  {
    id: "deepseek/deepseek-v2.5",
    name: "DeepSeek V2.5",
    provider: "deepseek",
    prefix: "deepseek/",
  },

  // Google models
  {
    id: "google/gemini-2.5-pro",
    name: "Gemini 2.5 Pro",
    provider: "google",
    prefix: "google/",
  },
  {
    id: "google/gemini-2.5-flash",
    name: "Gemini 2.5 Flash",
    provider: "google",
    prefix: "google/",
  },
  {
    id: "google/gemini-2.0-flash",
    name: "Gemini 2.0 Flash",
    provider: "google",
    prefix: "google/",
  },
];

export function getModelsByProvider(provider: string): ModelConfig[] {
  return MODELS_CONFIG.filter((model) => model.provider === provider);
}

/** Prefix segment for model IDs in the proxy (e.g. kiro connection → kr, not kiro). */
export function getModelIdPrefixForProvider(
  provider: string,
): string | undefined {
  const first = MODELS_CONFIG.find((m) => m.provider === provider);
  if (first?.prefix) return first.prefix.replace(/\/$/, "");
  return undefined;
}

export function getProviderFromModelId(modelId: string): string {
  const parts = modelId.split("/");
  if (parts.length === 2) {
    return parts[0];
  }
  return "other";
}

export function getModelName(modelId: string): string {
  // Always return the full model ID with prefix for better distinction
  return modelId;
}
