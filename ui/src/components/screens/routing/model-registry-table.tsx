import { TerminalSquare } from "lucide-react";
import { ProviderLogo } from "@/components/connections/ProviderLogo";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { getProviderLabel } from "@/lib/provider-registry";
import type { UiModel } from "./types";
import { getModelDisplayName } from "./model-display";
import { ModelTestButton, type ModelTestResult } from "./model-test-button";
import { ModelRoutingBadges, getModelRoutingLabel } from "./model-routing-badges";

interface ModelRegistryTableProps {
  models: UiModel[];
  testResults: Record<string, ModelTestResult>;
  onTestModel: (connectionId: string, modelId: string) => void;
  onOpenLogModal: (modelId: string) => void;
}

export function ModelRegistryTable({
  models,
  testResults,
  onTestModel,
  onOpenLogModal,
}: ModelRegistryTableProps) {
  return (
    <div className="hidden overflow-hidden rounded-lg border bg-card shadow-sm md:block">
      <Table>
        <TableHeader className="bg-muted/20">
          <TableRow className="hover:bg-transparent">
            <TableHead className="w-[170px] px-4 text-xs uppercase tracking-wide text-muted-foreground">Provider</TableHead>
            <TableHead className="text-xs uppercase tracking-wide text-muted-foreground">Model</TableHead>
            <TableHead className="text-xs uppercase tracking-wide text-muted-foreground">Model ID</TableHead>
            <TableHead className="text-xs uppercase tracking-wide text-muted-foreground">Routing</TableHead>
            <TableHead className="text-xs uppercase tracking-wide text-muted-foreground">Connections</TableHead>
            <TableHead className="w-[90px] text-xs uppercase tracking-wide text-muted-foreground">Status</TableHead>
            <TableHead className="w-[90px] text-right text-xs uppercase tracking-wide text-muted-foreground">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {models.map((model) => (
            <TableRow key={model.id} className="align-top hover:bg-muted/25">
              <TableCell className="px-4 py-4">
                <div className="flex items-center gap-2">
                  <ProviderLogo provider={model.provider} size={18} />
                  <span className="text-sm font-medium text-foreground/90">{getProviderLabel(model.provider)}</span>
                </div>
              </TableCell>
              <TableCell className="py-4">
                <p className="max-w-[220px] truncate font-medium" title={getModelDisplayName(model)}>
                  {getModelDisplayName(model)}
                </p>
              </TableCell>
              <TableCell className="py-4">
                <code className="block max-w-[300px] truncate rounded border border-border/50 bg-muted/40 px-2 py-1 font-mono text-xs text-muted-foreground" title={model.id}>
                  {model.id}
                </code>
              </TableCell>
              <TableCell className="py-4">
                <div className="space-y-1" title={getModelRoutingLabel(model)}>
                  <ModelRoutingBadges model={model} />
                  <p className="max-w-[180px] truncate text-[11px] text-muted-foreground">
                    {getModelRoutingLabel(model)}
                  </p>
                </div>
              </TableCell>
              <TableCell className="py-3">
                <div className="flex max-w-[420px] flex-wrap gap-2">
                  {(model.connections || []).length === 0 ? (
                    <span className="text-xs text-muted-foreground">No active connection</span>
                  ) : (
                    model.connections?.map((connection) => {
                      const key = `${connection.id}::${model.id}`;
                      return (
                        <div key={connection.id} className="flex h-8 items-center gap-1.5 rounded-md border border-border/60 bg-muted/35 px-2">
                          <span className="max-w-[140px] truncate text-xs font-medium text-foreground/90" title={connection.name}>
                            {connection.name}
                          </span>
                          <ModelTestButton
                            result={testResults[key]}
                            label={`Test ${model.id} on ${connection.name}`}
                            onClick={() => onTestModel(connection.id, model.id)}
                          />
                        </div>
                      );
                    })
                  )}
                </div>
              </TableCell>
              <TableCell className="py-4">
                <span className="inline-flex items-center gap-1.5 rounded-full bg-muted/60 px-2 py-1 text-xs font-medium">
                  <span className={model.isActive === false ? "h-1.5 w-1.5 rounded-full bg-destructive" : "h-1.5 w-1.5 rounded-full bg-emerald-500"} />
                  {model.isActive === false ? "Inactive" : "Active"}
                </span>
              </TableCell>
              <TableCell className="py-4 text-right">
                <Button
                  variant="outline"
                  size="icon"
                  onClick={() => onOpenLogModal(model.id)}
                  className="h-8 w-8 bg-background/60"
                  title={`View logs for ${model.id}`}
                  aria-label={`View logs for ${model.id}`}
                >
                  <TerminalSquare className="h-4 w-4" />
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
