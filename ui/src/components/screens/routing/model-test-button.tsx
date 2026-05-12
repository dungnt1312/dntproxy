import { CheckCircle2, Loader2, TestTube, XCircle } from "lucide-react";
import { Button } from "@/components/ui/button";

export interface ModelTestResult {
  status: "ok" | "error" | "loading";
  message?: string;
}

interface ModelTestButtonProps {
  result?: ModelTestResult;
  disabled?: boolean;
  label: string;
  onClick: () => void;
}

export function ModelTestButton({ result, disabled, label, onClick }: ModelTestButtonProps) {
  const icon =
    result?.status === "loading" ? (
      <Loader2 className="h-3.5 w-3.5 animate-spin" />
    ) : result?.status === "ok" ? (
      <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />
    ) : result?.status === "error" ? (
      <XCircle className="h-3.5 w-3.5 text-destructive" />
    ) : (
      <TestTube className="h-3.5 w-3.5" />
    );

  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={onClick}
      disabled={disabled || result?.status === "loading"}
      className="h-6 gap-1.5 rounded px-2 text-xs font-medium text-muted-foreground hover:bg-background/80 hover:text-foreground"
      title={result?.message || label}
      aria-label={label}
    >
      {icon}
      Test
    </Button>
  );
}
