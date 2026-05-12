import { ArrowDown, ArrowUp, Pin, Shuffle, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { ConnectionOption, UiModel } from "./types";
import type { ComboStep } from "./combo-step-builder";
import { getModelDisplayName } from "./model-display";

interface ComboStepListProps {
  steps: ComboStep[];
  connections: ConnectionOption[];
  models: UiModel[];
  onMove: (stepId: string, direction: "up" | "down") => void;
  onDelete: (stepId: string) => void;
}

export function ComboStepList({ steps, connections, models, onMove, onDelete }: ComboStepListProps) {
  if (steps.length === 0) {
    return (
      <div className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">
        No steps added yet.
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {steps.map((step, index) => {
        const connection = connections.find((item) => item.id === step.accountId);
        const model = models.find((item) => item.provider === step.provider && (item.modelId === step.model || item.id === `${step.provider}/${step.model}`));
        return (
          <div key={step.id} className="flex items-center gap-3 rounded-lg border bg-card p-3">
            <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-muted text-sm font-semibold">
              {index + 1}
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <Badge variant="outline" className="font-mono text-[10px]">{step.provider}</Badge>
                <span className="truncate text-sm font-medium">{model ? getModelDisplayName(model) : step.model}</span>
              </div>
              <div className="mt-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                {step.accountMode === "pinned" ? <Pin className="h-3 w-3" /> : <Shuffle className="h-3 w-3" />}
                {step.accountMode === "pinned" ? `Pinned to ${connection?.name || step.accountId}` : "Auto-select account"}
              </div>
            </div>
            <div className="flex shrink-0 gap-1">
              <Button variant="ghost" size="icon" onClick={() => onMove(step.id, "up")} disabled={index === 0} aria-label="Move step up">
                <ArrowUp className="h-4 w-4" />
              </Button>
              <Button variant="ghost" size="icon" onClick={() => onMove(step.id, "down")} disabled={index === steps.length - 1} aria-label="Move step down">
                <ArrowDown className="h-4 w-4" />
              </Button>
              <Button variant="ghost" size="icon" onClick={() => onDelete(step.id)} aria-label="Delete step">
                <Trash2 className="h-4 w-4 text-destructive" />
              </Button>
            </div>
          </div>
        );
      })}
    </div>
  );
}
