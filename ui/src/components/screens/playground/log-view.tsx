import { Clock, Copy, Terminal, Trash2, ChevronDown, ChevronRight } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { RequestLog } from "./types";

function formatDuration(ms?: number): string {
  if (!ms) return "-";
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

function formatCost(value?: number): string {
  if (!value) return "-";
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 6,
  }).format(value);
}

function prettyJSON(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text);
}

type Props = {
  logs: RequestLog[];
  selectedLogId: string | null;
  onSelectLog: (id: string) => void;
  onClear: () => void;
};

export function PlaygroundLogView({ logs, selectedLogId, onSelectLog, onClear }: Props) {
  if (logs.length === 0) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-3 text-center p-8">
        <Terminal className="h-12 w-12 text-muted-foreground/50" />
        <div>
          <p className="font-medium">No logs yet</p>
          <p className="text-sm text-muted-foreground mt-1">Send a chat request to see detailed logs here</p>
        </div>
      </div>
    );
  }

  const selectedLog = selectedLogId ? logs.find((l) => l.id === selectedLogId) : null;

  return (
    <>
      <div className="flex items-center justify-between px-4 py-2 border-b">
        <p className="text-xs text-muted-foreground">{logs.length} request{logs.length !== 1 ? "s" : ""} logged</p>
        <Button variant="outline" size="sm" onClick={onClear} className="text-xs h-7">
          <Trash2 className="mr-1 h-3 w-3" />
          Clear
        </Button>
      </div>

      <div className="flex flex-1 min-h-0 flex-col md:flex-row">
        <ScrollArea className="max-h-64 flex-1 border-b md:max-h-none md:max-w-md md:border-b-0 md:border-r">
          <div className="p-2 space-y-2">
            {logs.map((log) => (
              <button
                key={log.id}
                onClick={() => onSelectLog(log.id)}
                className={`w-full text-left p-3 rounded-lg border transition-colors ${
                  selectedLogId === log.id
                    ? "border-emerald-500 bg-emerald-50 dark:bg-emerald-950/30"
                    : "hover:bg-accent/50"
                }`}
              >
                <div className="flex items-center justify-between mb-1">
                  <Badge variant={log.status === "success" ? "default" : "destructive"} className="text-[10px] h-4">
                    {log.status === "success" ? "200 OK" : "Error"}
                  </Badge>
                  <div className="flex items-center gap-1 text-xs text-muted-foreground">
                    <Clock className="h-3 w-3" />
                    {formatDuration(log.durationMs)}
                  </div>
                </div>
                <p className="text-xs font-medium truncate">{log.model}</p>
              </button>
            ))}
          </div>
        </ScrollArea>

        <ScrollArea className="flex-1">
          {selectedLog ? (
            <div className="p-4 space-y-4">
              <div className="flex items-center justify-between">
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <Badge variant={selectedLog.status === "success" ? "default" : "destructive"}>
                      {selectedLog.status === "success" ? "Success" : "Failed"}
                    </Badge>
                    <span className="text-xs font-mono">{selectedLog.model}</span>
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {new Date(selectedLog.timestamp).toLocaleString()} · {formatDuration(selectedLog.durationMs)}
                  </p>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  aria-label="Copy selected log"
                  onClick={() => copyToClipboard(JSON.stringify(selectedLog, null, 2))}
                  className="text-xs"
                >
                  <Copy className="mr-1 h-3 w-3" />
                  Copy
                </Button>
              </div>

              {selectedLog.totalTokens !== undefined && (
                <div className="grid grid-cols-3 gap-3">
                  <Card><CardContent className="pt-4 text-center"><p className="text-2xl font-bold">{selectedLog.inputTokens || "-"}</p><p className="text-xs text-muted-foreground">Input tokens</p></CardContent></Card>
                  <Card><CardContent className="pt-4 text-center"><p className="text-2xl font-bold">{selectedLog.outputTokens || "-"}</p><p className="text-xs text-muted-foreground">Output tokens</p></CardContent></Card>
                  <Card><CardContent className="pt-4 text-center"><p className="text-2xl font-bold">{formatCost(selectedLog.costTotal)}</p><p className="text-xs text-muted-foreground">Cost</p></CardContent></Card>
                </div>
              )}

              <Collapsible defaultOpen>
                <CollapsibleTrigger className="flex items-center gap-2 w-full text-sm font-medium"><ChevronDown className="h-4 w-4" />Request Body</CollapsibleTrigger>
                <CollapsibleContent className="mt-2"><pre className="bg-muted p-3 rounded-lg text-xs overflow-x-auto max-h-[400px]">{prettyJSON(selectedLog.requestBody)}</pre></CollapsibleContent>
              </Collapsible>

              {selectedLog.responseBody && (
                <Collapsible>
                  <CollapsibleTrigger className="flex items-center gap-2 w-full text-sm font-medium"><ChevronRight className="h-4 w-4" />Response</CollapsibleTrigger>
                  <CollapsibleContent className="mt-2"><pre className="bg-muted p-3 rounded-lg text-xs overflow-x-auto max-h-[400px]">{prettyJSON(selectedLog.responseBody)}</pre></CollapsibleContent>
                </Collapsible>
              )}

              {selectedLog.error && (
                <Card className="border-destructive">
                  <CardContent className="pt-4">
                    <div className="flex items-start gap-2"><Badge variant="destructive">Error</Badge><pre className="text-xs text-destructive whitespace-pre-wrap">{selectedLog.error}</pre></div>
                  </CardContent>
                </Card>
              )}
            </div>
          ) : (
            <div className="flex flex-1 items-center justify-center text-muted-foreground text-sm">Select a log to view details</div>
          )}
        </ScrollArea>
      </div>
    </>
  );
}
