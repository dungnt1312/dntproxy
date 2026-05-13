import { getProviderLabel } from "@/lib/provider-registry";
import type { ConnectionOption, UiModel } from "./types";
import { getModelDisplayName } from "./model-display";

const PROVIDER_PREFIX_LABELS: Record<string, string> = {
  kr: "Kiro",
  oai: "OpenAI",
  glm: "GLM",
  minimax: "MiniMax",
  qwen: "Qwen",
  anthropic: "Anthropic",
  gemini: "Gemini",
};

export function parseRoutingTarget(target: string) {
  const [modelPart, connectionId] = target.split("@");
  const slashIndex = modelPart.indexOf("/");
  const provider = slashIndex === -1 ? "" : modelPart.slice(0, slashIndex);
  const model = slashIndex === -1 ? modelPart : modelPart.slice(slashIndex + 1);
  return { provider, model, connectionId: connectionId && connectionId !== "auto" ? connectionId : "" };
}

export function getTargetDisplay(target: string, models: UiModel[], connections: ConnectionOption[]): string {
  const parsed = parseRoutingTarget(target);
  const exact = models.find((model) => model.id === target || model.id === `${parsed.provider}/${parsed.model}`);
  const provider = PROVIDER_PREFIX_LABELS[parsed.provider] || getProviderLabel(parsed.provider);
  const modelLabel = exact ? getModelDisplayName(exact) : parsed.model || target;
  const account = parsed.connectionId
    ? connections.find((connection) => connection.id === parsed.connectionId)?.name || parsed.connectionId
    : "";

  return account ? `${provider} / ${modelLabel} @ ${account}` : `${provider} / ${modelLabel}`;
}

export function providerPrefixToProvider(prefix: string): string {
  if (prefix === "kr") return "kiro";
  if (prefix === "oai") return "openai";
  return prefix;
}

export function providerToRoutingPrefix(provider: string): string {
  if (provider === "kiro") return "kr";
  if (provider === "openai") return "oai";
  return provider;
}
