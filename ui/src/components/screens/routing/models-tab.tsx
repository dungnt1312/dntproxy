import { useMemo, useState } from "react";
import { Box, Layers, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { goApi } from "@/lib/go-api";
import type { UiModel } from "./types";
import { getModelSearchText, isRegistryModel, sortModels } from "./model-display";
import { ModelRegistryMobileList } from "./model-registry-mobile-list";
import { ModelRegistryTable } from "./model-registry-table";
import { type ModelTestResult } from "./model-test-button";
import { RoutingEmptyState } from "./routing-empty-state";
import { RoutingToolbar } from "./routing-toolbar";

export interface ModelsTabProps {
  models: UiModel[];
  loading: boolean;
  hasLoadError?: boolean;
  onOpenLogModal: (modelId: string) => void;
}

export default function ModelsTab({ models, loading, hasLoadError, onOpenLogModal }: ModelsTabProps) {
  const [search, setSearch] = useState("");
  const [testResults, setTestResults] = useState<Record<string, ModelTestResult>>({});
  const q = search.trim().toLowerCase();

  const registryModels = useMemo(() => sortModels(models.filter(isRegistryModel)), [models]);
  const filteredModels = useMemo(() => {
    if (!q) return registryModels;
    return registryModels.filter((model) => getModelSearchText(model).includes(q));
  }, [q, registryModels]);

  const providerCount = useMemo(
    () => new Set(registryModels.map((model) => model.provider)).size,
    [registryModels],
  );

  async function handleTestModel(connectionId: string, modelId: string) {
    const key = `${connectionId}::${modelId}`;
    setTestResults((prev) => ({ ...prev, [key]: { status: "loading" } }));
    try {
      const res = await goApi.testModel(connectionId, modelId);
      setTestResults((prev) => ({
        ...prev,
        [key]: { status: res.status === "ok" ? "ok" : "error", message: res.message },
      }));
    } catch (error) {
      const message = error instanceof Error ? error.message : "Model test failed";
      setTestResults((prev) => ({ ...prev, [key]: { status: "error", message } }));
      toast.error(message);
    }
  }

  return (
    <div className="space-y-4 outline-none">
      <RoutingToolbar
        search={search}
        onSearchChange={setSearch}
        placeholder="Search models, providers, IDs, or connections..."
        summary={`${filteredModels.length} of ${registryModels.length} models across ${providerCount} providers`}
      />

      {loading ? (
        <div className="flex items-center justify-center rounded-lg border bg-card p-16 text-sm text-muted-foreground">
          <Loader2 className="mr-2 h-5 w-5 animate-spin" />
          Loading models...
        </div>
      ) : filteredModels.length === 0 ? (
        <RoutingEmptyState
          icon={q ? <Box className="h-5 w-5 text-muted-foreground" /> : <Layers className="h-5 w-5 text-muted-foreground" />}
          title={q ? "No matching models" : hasLoadError ? "Models could not load" : "No registry models"}
          description={
            q
              ? `No models match "${search.trim()}".`
              : hasLoadError
                ? "Retry loading routing data from the error banner above."
                : "Connect a provider or detect models to populate the registry."
          }
        />
      ) : (
        <>
          <ModelRegistryTable
            models={filteredModels}
            testResults={testResults}
            onTestModel={handleTestModel}
            onOpenLogModal={onOpenLogModal}
          />
          <ModelRegistryMobileList
            models={filteredModels}
            testResults={testResults}
            onTestModel={handleTestModel}
            onOpenLogModal={onOpenLogModal}
          />
        </>
      )}
    </div>
  );
}
