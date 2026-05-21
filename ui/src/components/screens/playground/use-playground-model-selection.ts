import { useCallback, useMemo, useState } from "react";
import type { Model } from "./types";
import { buildPlaygroundModelId, getSelectableModelValue } from "./stream-utils";

export function usePlaygroundModelSelection(models: Model[]) {
  const [selectedProvider, setSelectedProvider] = useState("");
  const [selectedModel, setSelectedModel] = useState("");
  const [selectedAccount, setSelectedAccount] = useState("auto");

  const finalModelString = useMemo(() => {
    return buildPlaygroundModelId(selectedProvider, selectedModel, selectedAccount);
  }, [selectedAccount, selectedModel, selectedProvider]);

  const displayProvider = (m: Model) => m.routePrefix || m.provider

  const selectedModelDetails = useMemo(() => {
    return models.find((model) => (
      displayProvider(model) === selectedProvider && getSelectableModelValue(model) === selectedModel
    ));
  }, [models, selectedModel, selectedProvider]);

  const supportsImages = useMemo(() => {
    if (!selectedProvider || selectedProvider === "combo" || selectedProvider === "alias") return true;
    const capabilities = selectedModelDetails?.capabilities;
    return !Array.isArray(capabilities) || capabilities.includes("vision");
  }, [selectedModelDetails, selectedProvider]);

  const initializeSelection = useCallback((availableModels: Model[]) => {
    if (availableModels.length === 0) return;
    const first = availableModels[0];
    setSelectedProvider(displayProvider(first));
    setSelectedModel(getSelectableModelValue(first));
    setSelectedAccount("auto");
  }, []);

  const handleProviderChange = useCallback((provider: string) => {
    setSelectedProvider(provider);
    setSelectedAccount("auto");
    const firstModel = models.find((model) => displayProvider(model) === provider);
    setSelectedModel(firstModel ? getSelectableModelValue(firstModel) : "");
  }, [models]);

  return {
    selectedProvider,
    selectedModel,
    selectedAccount,
    setSelectedModel,
    setSelectedAccount,
    finalModelString,
    supportsImages,
    initializeSelection,
    handleProviderChange,
  };
}
