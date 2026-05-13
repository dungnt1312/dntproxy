import { AlertTriangle, GitBranch, Pin } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { ConnectionOption, UiModel } from "./types";
import { getTargetDisplay, parseRoutingTarget } from "./routing-format";

interface ComboTargetChainProps {
  targets: string[];
  models: UiModel[];
  connections: ConnectionOption[];
}

export function ComboTargetChain({ targets, models, connections }: ComboTargetChainProps) {
  return (
    <ol className="mt-3 grid gap-2 lg:grid-cols-2 xl:grid-cols-3">
      {targets.map((target, index) => {
        const parsed = parseRoutingTarget(target);
        const connection = parsed.connectionId
          ? connections.find((item) => item.id === parsed.connectionId)
          : undefined;
        const hasBadPin = Boolean(parsed.connectionId && (!connection || connection.isActive === false));

        return (
          <li
            key={`${target}-${index}`}
            className="min-w-0 rounded-md border bg-muted/25 px-3 py-2"
            title={target}
          >
            <div className="flex min-w-0 items-center gap-2">
              <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded bg-background text-[11px] font-semibold text-muted-foreground">
                {index + 1}
              </span>
              <GitBranch className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
              <span className="min-w-0 truncate text-xs font-medium">
                {getTargetDisplay(target, models, connections)}
              </span>
            </div>
            <div className="mt-1 flex min-w-0 items-center gap-1.5 pl-7">
              <code className="min-w-0 truncate font-mono text-[11px] text-muted-foreground">{target}</code>
              {parsed.connectionId && !hasBadPin && (
                <Badge variant="outline" className="h-5 gap-1 px-1.5 text-[10px]">
                  <Pin className="h-3 w-3" aria-hidden="true" />
                  Pinned
                </Badge>
              )}
              {hasBadPin && (
                <Badge variant="destructive" className="h-5 gap-1 px-1.5 text-[10px]">
                  <AlertTriangle className="h-3 w-3" aria-hidden="true" />
                  Missing
                </Badge>
              )}
            </div>
          </li>
        );
      })}
    </ol>
  );
}
