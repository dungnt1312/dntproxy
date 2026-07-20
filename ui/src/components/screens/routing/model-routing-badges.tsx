import { Link2, Shuffle, Pin } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { UiModel } from "./types";

interface ModelRoutingBadgesProps {
  model: UiModel;
  compact?: boolean;
}

export function getModelConnectionCount(model: UiModel): number {
  if (model.connections?.length) return model.connections.length;
  return model.connectionId ? 1 : 0;
}

export function getModelRoutingLabel(model: UiModel): string {
  const count = getModelConnectionCount(model);
  if (model.connectionId) return `Pinned to ${model.connectionName || model.connectionId}`;
  if (count > 1) return `Auto-select across ${count} connections`;
  if (count === 1) return `Single connection: ${model.connections?.[0]?.name || model.connectionName || "configured"}`;
  return "No active route";
}

export function ModelRoutingBadges({ model, compact }: ModelRoutingBadgesProps) {
  const count = getModelConnectionCount(model);

  if (model.connectionId) {
    return (
      <Badge variant="outline" className="border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300" title={getModelRoutingLabel(model)}>
        <Pin className="h-3 w-3" />
        {compact ? "Pinned" : "Pinned account"}
      </Badge>
    );
  }

  if (count > 1) {
    return (
      <Badge variant="outline" className="border-blue-500/30 bg-blue-500/10 text-blue-700 dark:text-blue-300" title={getModelRoutingLabel(model)}>
        <Shuffle className="h-3 w-3" />
        {compact ? `${count} routes` : `Auto-select · ${count}`}
      </Badge>
    );
  }

  if (count === 1) {
    return (
      <Badge variant="outline" className="border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300" title={getModelRoutingLabel(model)}>
        <Link2 className="h-3 w-3" />
        {compact ? "1 route" : "Single route"}
      </Badge>
    );
  }

  return (
    <Badge variant="outline" className="border-destructive/30 bg-destructive/10 text-destructive" title={getModelRoutingLabel(model)}>
      No route
    </Badge>
  );
}
