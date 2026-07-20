import { TerminalSquare } from "lucide-react";
import { ProviderLogo } from "@/components/connections/ProviderLogo";
import { Button } from "@/components/ui/button";
import { getProviderLabel } from "@/lib/provider-registry";
import type { UiModel } from "./types";
import { getModelDisplayName } from "./model-display";
import { ModelTestButton, type ModelTestResult } from "./model-test-button";
import { ModelRoutingBadges, getModelRoutingLabel } from "./model-routing-badges";

interface ModelRegistryCardGridProps {
  models: UiModel[];
  testResults: Record<string, ModelTestResult>;
  onTestModel: (connectionId: string, modelId: string) => void;
  onOpenLogModal: (modelId: string) => void;
}

export function ModelRegistryCardGrid({
  models,
  testResults,
  onTestModel,
  onOpenLogModal,
}: ModelRegistryCardGridProps) {
  return (
    <div className="hidden grid-cols-1 gap-4 md:grid xl:grid-cols-2 2xl:grid-cols-3">
      {models.map((model) => (
        <div key={model.id} className="rounded-xl border bg-card p-4 shadow-sm transition-colors hover:bg-muted/15">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 space-y-2">
              <div className="flex items-center gap-2">
                <ProviderLogo provider={model.provider} size={18} />
                <span className="text-xs font-medium text-muted-foreground">
                  {getProviderLabel(model.provider)}
                </span>
              </div>
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold" title={getModelDisplayName(model)}>
                  {getModelDisplayName(model)}
                </p>
                <code className="mt-1 block truncate font-mono text-xs text-muted-foreground" title={model.id}>
                  {model.id}
                </code>
              </div>
            </div>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => onOpenLogModal(model.id)}
              className="h-8 w-8 shrink-0"
              title={`View logs for ${model.id}`}
              aria-label={`View logs for ${model.id}`}
            >
              <TerminalSquare className="h-4 w-4" />
            </Button>
          </div>

          <div className="mt-4 flex flex-wrap items-center gap-2">
            <ModelRoutingBadges model={model} />
            <span className="inline-flex items-center gap-1.5 rounded-full bg-muted/60 px-2 py-1 text-xs font-medium">
              <span className={model.isActive === false ? "h-1.5 w-1.5 rounded-full bg-destructive" : "h-1.5 w-1.5 rounded-full bg-emerald-500"} />
              {model.isActive === false ? "Inactive" : "Active"}
            </span>
          </div>
          <p className="mt-2 text-xs text-muted-foreground">{getModelRoutingLabel(model)}</p>

          <div className="mt-4 border-t pt-3">
            <div className="mb-2 flex items-center justify-between gap-2">
              <span className="text-xs font-medium text-muted-foreground">Connections</span>
              <span className="text-[11px] text-muted-foreground">
                {(model.connections || []).length || 0} available
              </span>
            </div>
            {(model.connections || []).length === 0 ? (
              <p className="text-xs text-muted-foreground">No active connection can serve this model.</p>
            ) : (
              <div className="space-y-2">
                {model.connections?.slice(0, 4).map((connection) => {
                  const key = `${connection.id}::${model.id}`;
                  return (
                    <div key={connection.id} className="flex items-center justify-between gap-2 rounded-md border bg-muted/25 px-2 py-1.5">
                      <span className="truncate text-xs font-medium" title={connection.name}>{connection.name}</span>
                      <ModelTestButton
                        result={testResults[key]}
                        label={`Test ${model.id} on ${connection.name}`}
                        onClick={() => onTestModel(connection.id, model.id)}
                      />
                    </div>
                  );
                })}
                {(model.connections?.length || 0) > 4 && (
                  <p className="text-[11px] text-muted-foreground">+{(model.connections?.length || 0) - 4} more connections</p>
                )}
              </div>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
