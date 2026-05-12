import { Link2, TerminalSquare, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { ConnectionOption, UiModel } from "./types";
import { getTargetDisplay } from "./routing-format";

interface AliasesListProps {
  aliases: Array<[string, string]>;
  models: UiModel[];
  connections?: ConnectionOption[];
  onOpenLogModal: (alias: string) => void;
  onDelete: (alias: string) => void;
}

export function AliasesList({ aliases, models, connections = [], onOpenLogModal, onDelete }: AliasesListProps) {
  return (
    <div className="overflow-hidden rounded-lg border bg-card">
      <div className="divide-y">
        {aliases.map(([alias, target]) => (
          <div key={alias} className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <Link2 className="h-4 w-4 text-muted-foreground" />
                <p className="truncate text-sm font-semibold">{alias}</p>
              </div>
              <p className="mt-1 truncate text-sm text-muted-foreground" title={target}>
                {getTargetDisplay(target, models, connections)}
              </p>
              <code className="mt-2 block truncate rounded bg-muted px-2 py-1 font-mono text-xs text-muted-foreground">
                {target}
              </code>
            </div>
            <div className="flex shrink-0 gap-1">
              <Button variant="ghost" size="icon" onClick={() => onOpenLogModal(alias)} aria-label={`View logs for alias ${alias}`}>
                <TerminalSquare className="h-4 w-4" />
              </Button>
              <Button variant="ghost" size="icon" onClick={() => onDelete(alias)} aria-label={`Delete alias ${alias}`}>
                <Trash2 className="h-4 w-4 text-destructive" />
              </Button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
