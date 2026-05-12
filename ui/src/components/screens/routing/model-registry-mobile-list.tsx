import { TerminalSquare } from "lucide-react";
import { ProviderLogo } from "@/components/connections/ProviderLogo";
import { Button } from "@/components/ui/button";
import { getProviderLabel } from "@/lib/provider-registry";
import type { UiModel } from "./types";
import { getModelDisplayName } from "./model-display";
import { ModelTestButton, type ModelTestResult } from "./model-test-button";

interface ModelRegistryMobileListProps {
  models: UiModel[];
  testResults: Record<string, ModelTestResult>;
  onTestModel: (connectionId: string, modelId: string) => void;
  onOpenLogModal: (modelId: string) => void;
}

export function ModelRegistryMobileList({
  models,
  testResults,
  onTestModel,
  onOpenLogModal,
}: ModelRegistryMobileListProps) {
  return (
    <div className="space-y-3 md:hidden">
      {models.map((model) => (
        <div key={model.id} className="rounded-lg border bg-card p-4 shadow-sm">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <div className="mb-2 flex items-center gap-2">
                <ProviderLogo provider={model.provider} size={18} />
                <span className="text-xs text-muted-foreground">{getProviderLabel(model.provider)}</span>
              </div>
              <p className="truncate text-sm font-semibold" title={getModelDisplayName(model)}>
                {getModelDisplayName(model)}
              </p>
            </div>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => onOpenLogModal(model.id)}
              className="h-8 w-8 shrink-0"
              aria-label={`View logs for ${model.id}`}
            >
              <TerminalSquare className="h-4 w-4" />
            </Button>
          </div>
          <code className="mt-3 block break-all rounded bg-muted px-2 py-1.5 font-mono text-xs text-muted-foreground">
            {model.id}
          </code>
          <div className="mt-3 flex flex-wrap gap-2">
            <span className="inline-flex items-center gap-1.5 rounded-full bg-muted/60 px-2 py-1 text-xs font-medium">
              <span className={model.isActive === false ? "h-1.5 w-1.5 rounded-full bg-destructive" : "h-1.5 w-1.5 rounded-full bg-emerald-500"} />
              {model.isActive === false ? "Inactive" : "Active"}
            </span>
            {(model.connections || []).length === 0 && (
              <span className="text-xs text-muted-foreground">No active connection</span>
            )}
          </div>
          {(model.connections || []).length > 0 && (
            <div className="mt-3 space-y-2">
              {model.connections?.map((connection) => {
                const key = `${connection.id}::${model.id}`;
                return (
                  <div key={connection.id} className="flex items-center justify-between gap-2 rounded-md border bg-muted/30 p-2">
                    <span className="truncate text-xs font-medium">{connection.name}</span>
                    <ModelTestButton
                      result={testResults[key]}
                      label={`Test ${model.id} on ${connection.name}`}
                      onClick={() => onTestModel(connection.id, model.id)}
                    />
                  </div>
                );
              })}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
