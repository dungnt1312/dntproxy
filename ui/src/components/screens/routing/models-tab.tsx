import { useMemo, useState } from "react";
import { Box, Layers, LayoutGrid, Loader2, Table2 } from "lucide-react";
import { toast } from "sonner";
import { goApi } from "@/lib/go-api";
import { getProviderLabel } from "@/lib/provider-registry";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import type { UiModel } from "./types";
import { getModelSearchText, isRegistryModel, sortModels } from "./model-display";
import { ModelRegistryMobileList } from "./model-registry-mobile-list";
import { ModelRegistryTable } from "./model-registry-table";
import { ModelRegistryCardGrid } from "./model-registry-card-grid";
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
  const [viewMode, setViewMode] = useState<"table" | "grid">("table");
  const [providerFilter, setProviderFilter] = useState("all");
  const [connectionFilter, setConnectionFilter] = useState("all");
  const q = search.trim().toLowerCase();

  const registryModels = useMemo(() => sortModels(models.filter(isRegistryModel)), [models]);
  const providerOptions = useMemo(
    () => Array.from(new Set(registryModels.map((model) => model.provider))).sort(),
    [registryModels],
  );
  const connectionOptions = useMemo(() => {
    const options = new Map<string, string>();
    registryModels.forEach((model) => {
      model.connections?.forEach((connection) => options.set(connection.id, connection.name));
      if (model.connectionId) options.set(model.connectionId, model.connectionName || model.connectionId);
    });
    return Array.from(options.entries()).sort((a, b) => a[1].localeCompare(b[1]));
  }, [registryModels]);

  const filteredModels = useMemo(() => {
    return registryModels.filter((model) => {
      if (q && !getModelSearchText(model).includes(q)) return false;
      if (providerFilter !== "all" && model.provider !== providerFilter) return false;
      if (connectionFilter !== "all") {
        const hasConnection = model.connectionId === connectionFilter || model.connections?.some((connection) => connection.id === connectionFilter);
        if (!hasConnection) return false;
      }
      return true;
    });
  }, [q, registryModels, providerFilter, connectionFilter]);

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
        action={
          <div className="flex flex-wrap items-center gap-2">
            <Select value={providerFilter} onValueChange={setProviderFilter}>
              <SelectTrigger className="h-9 w-[150px] text-xs">
                <SelectValue placeholder="Provider" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All providers</SelectItem>
                {providerOptions.map((provider) => (
                  <SelectItem key={provider} value={provider}>
                    {getProviderLabel(provider)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select value={connectionFilter} onValueChange={setConnectionFilter}>
              <SelectTrigger className="h-9 w-[180px] text-xs">
                <SelectValue placeholder="Connection" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All connections</SelectItem>
                {connectionOptions.map(([id, name]) => (
                  <SelectItem key={id} value={id}>
                    {name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <ToggleGroup
              type="single"
              value={viewMode}
              onValueChange={(value) => value && setViewMode(value as "table" | "grid")}
              variant="outline"
              size="sm"
              className="hidden md:flex"
            >
              <ToggleGroupItem value="table" aria-label="Table view" className="h-9 px-3">
                <Table2 className="h-4 w-4" />
              </ToggleGroupItem>
              <ToggleGroupItem value="grid" aria-label="Grid view" className="h-9 px-3">
                <LayoutGrid className="h-4 w-4" />
              </ToggleGroupItem>
            </ToggleGroup>
          </div>
        }
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
          {viewMode === "table" ? (
            <ModelRegistryTable
              models={filteredModels}
              testResults={testResults}
              onTestModel={handleTestModel}
              onOpenLogModal={onOpenLogModal}
            />
          ) : (
            <ModelRegistryCardGrid
              models={filteredModels}
              testResults={testResults}
              onTestModel={handleTestModel}
              onOpenLogModal={onOpenLogModal}
            />
          )}
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
