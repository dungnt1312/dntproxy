import { AlertCircle, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

interface RoutingErrorStateProps {
  errors: string[];
  onRetry: () => void;
}

export function RoutingErrorState({ errors, onRetry }: RoutingErrorStateProps) {
  if (errors.length === 0) return null;

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm sm:flex-row sm:items-start sm:justify-between">
      <div className="flex gap-3">
        <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
        <div>
          <p className="font-medium text-destructive">Some routing data failed to load</p>
          <p className="mt-1 text-muted-foreground">{errors.join(" ")}</p>
        </div>
      </div>
      <Button variant="outline" size="sm" onClick={onRetry} className="gap-2 self-start">
        <RefreshCw className="h-3.5 w-3.5" />
        Retry
      </Button>
    </div>
  );
}
