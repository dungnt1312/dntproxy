import { PROVIDER_ORDER } from "@/lib/provider-registry";
import type { UiModel } from "./types";

export function isRegistryModel(model: UiModel): boolean {
  return model.provider !== "alias" && model.provider !== "combo";
}

export function getModelDisplayName(model: UiModel): string {
  return model.displayName || model.name || model.modelId || model.id || "Unnamed model";
}

export function getModelSearchText(model: UiModel): string {
  const connections = model.connections?.map((connection) => connection.name).join(" ") || "";
  return [
    model.id,
    model.modelId,
    model.name,
    model.displayName,
    model.provider,
    model.connectionName,
    connections,
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

export function sortModels(models: UiModel[]): UiModel[] {
  return [...models].sort((a, b) => {
    const providerDelta = getProviderSort(a.provider) - getProviderSort(b.provider);
    if (providerDelta !== 0) return providerDelta;
    return getModelDisplayName(a).localeCompare(getModelDisplayName(b));
  });
}

function getProviderSort(provider: string): number {
  const index = PROVIDER_ORDER.indexOf(provider as (typeof PROVIDER_ORDER)[number]);
  return index === -1 ? PROVIDER_ORDER.length : index;
}
