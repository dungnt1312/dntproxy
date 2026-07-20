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
  other: { name: "Other", icon: "OT", color: "text-gray-400" },
};

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
