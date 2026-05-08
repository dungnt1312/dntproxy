import { AlertTriangle } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import { StatusBadge, formatDateTime, formatLatency } from "./helpers";
import { PayloadViewer } from "./PayloadViewer";
import type { LogEntry } from "@/types/logs";

export interface LogDetailSheetProps {
  log: LogEntry | undefined;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function LogDetailSheet({ log, open, onOpenChange }: LogDetailSheetProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full sm:max-w-xl overflow-y-auto p-0">
        {log ? (
          <div className="flex flex-col h-full">
            <SheetHeader className="p-6 border-b shrink-0">
              <SheetTitle>Log Detail</SheetTitle>
              <SheetDescription>Detailed request and response information</SheetDescription>
            </SheetHeader>

            <div className="p-6 space-y-6 flex-1 overflow-y-auto">
              {/* Overview */}
              <div className="space-y-3">
                <h3 className="font-semibold text-sm">Overview</h3>
                <div className="grid grid-cols-2 gap-3 text-sm border rounded-lg p-4 bg-muted/20">
                  <div>
                    <span className="text-muted-foreground text-xs block mb-1">Time</span>
                    <span className="font-medium">{formatDateTime(log.timestamp)}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground text-xs block mb-1">Status</span>
                    <span className="flex items-center">
                      <StatusBadge status={log.statusCode} level={log.level} />
                    </span>
                  </div>
                  <div>
                    <span className="text-muted-foreground text-xs block mb-1">Provider & Model</span>
                    <div className="font-medium truncate" title={log.model || ""}>
                      {log.provider || "—"} / {log.model || "—"}
                    </div>
                  </div>
                  <div>
                    <span className="text-muted-foreground text-xs block mb-1">Connection</span>
                    <span className="font-medium">{log.connectionName || log.connectionId || "—"}</span>
                  </div>
                  {log.durationMs != null && (
                    <div>
                      <span className="text-muted-foreground text-xs block mb-1">Latency</span>
                      <span className="font-medium">{formatLatency(log.durationMs)}</span>
                    </div>
                  )}
                  {log.totalTokens != null && (
                    <div>
                      <span className="text-muted-foreground text-xs block mb-1">Tokens Usage</span>
                      <span className="font-medium">{log.totalTokens.toLocaleString()} Total</span>
                    </div>
                  )}
                  {(log.method || log.path) && (
                    <div className="col-span-2">
                      <span className="text-muted-foreground text-xs block mb-1">Request Path</span>
                      <div className="font-mono text-xs break-all bg-muted p-2 rounded border">
                        {log.method && <span className="font-bold mr-2">{log.method}</span>}
                        {log.path || "—"}
                      </div>
                    </div>
                  )}
                </div>
              </div>

              {/* Error Details */}
              {log.error && (
                <div className="space-y-2">
                  <h3 className="font-semibold text-sm text-rose-600 flex items-center gap-1.5">
                    <AlertTriangle className="h-4 w-4" /> Error Summary
                  </h3>
                  <div className="rounded-lg bg-rose-50 dark:bg-rose-950/30 border border-rose-200 dark:border-rose-800 p-3">
                    <p className="text-xs text-rose-700 dark:text-rose-400 whitespace-pre-wrap break-all">
                      {log.error}
                    </p>
                  </div>
                </div>
              )}

              {/* Message Details */}
              {log.message && !log.path && (
                <div className="space-y-2">
                  <h3 className="font-semibold text-sm">Message</h3>
                  <div className="rounded-lg border p-3 bg-muted/20">
                    <p className="text-sm">{log.message}</p>
                  </div>
                </div>
              )}

              {/* Collapsible JSON Sections */}
              {(log.requestBody || log.responseBody || log.metadataJson) && (
                <div className="space-y-3 pt-2">
                  <h3 className="font-semibold text-sm mb-2">Payloads</h3>
                  {(() => {
                    let meta: any = {};
                    if (log.metadataJson && typeof log.metadataJson === "string") {
                      try {
                        meta = JSON.parse(log.metadataJson);
                      } catch {}
                    } else if (log.metadataJson && typeof log.metadataJson === "object") {
                      meta = log.metadataJson;
                    }

                    return (
                      <>
                        <PayloadViewer label="Request Headers" rawContent={meta?.requestHeaders} />
                        <PayloadViewer label="Request Body" rawContent={log.requestBody} />
                        <PayloadViewer label="Response Headers" rawContent={meta?.responseHeaders} />
                        <PayloadViewer label="Response Body" rawContent={log.responseBody} />
                        <PayloadViewer label="Metadata" rawContent={log.metadataJson} />
                      </>
                    );
                  })()}
                </div>
              )}
            </div>
          </div>
        ) : (
          <div className="p-6 space-y-4">
            <Skeleton className="h-8 w-1/3" />
            <Skeleton className="h-[200px] w-full" />
            <Skeleton className="h-[150px] w-full" />
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
