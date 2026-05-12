import { useEffect, useState } from "react";
import { AlertTriangle } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import {
  StatusBadge,
  formatCompressionRatio,
  formatDateTime,
  formatLatency,
  formatTokenCount,
  getCompressionMetadata,
} from "./helpers";
import { PayloadViewer } from "./PayloadViewer";
import { goApi } from "@/lib/go-api";
import type { LogEntry } from "@/types/logs";

export interface LogDetailSheetProps {
  log: LogEntry | undefined;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function LogDetailSheet({ log, open, onOpenChange }: LogDetailSheetProps) {
  const [fullLog, setFullLog] = useState<LogEntry | null>(null);
  const [loadingDetail, setLoadingDetail] = useState(false);

  // Fetch full detail (with bodies) when sheet opens
  useEffect(() => {
    if (!open || !log?.id) {
      setFullLog(null);
      return;
    }
    setLoadingDetail(true);
    goApi.getLogDetail(log.id)
      .then((detail) => {
        if (detail) setFullLog(detail);
        else setFullLog(log); // fallback to list entry
      })
      .catch(() => setFullLog(log))
      .finally(() => setLoadingDetail(false));
  }, [open, log?.id]);

  const displayLog = fullLog || log;
  const compression = displayLog ? getCompressionMetadata(displayLog) : null;

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full sm:max-w-xl overflow-y-auto p-0">
        {displayLog ? (
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
                    <span className="font-medium">{formatDateTime(displayLog.timestamp)}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground text-xs block mb-1">Status</span>
                    <span className="flex items-center">
                      <StatusBadge status={displayLog.statusCode} level={displayLog.level} />
                    </span>
                  </div>
                  <div>
                    <span className="text-muted-foreground text-xs block mb-1">Provider & Model</span>
                    <div className="font-medium truncate" title={displayLog.model || ""}>
                      {displayLog.provider || "—"} / {displayLog.model || "—"}
                    </div>
                  </div>
                  <div>
                    <span className="text-muted-foreground text-xs block mb-1">Connection</span>
                    <span className="font-medium">{displayLog.connectionName || displayLog.connectionId || "—"}</span>
                  </div>
                  {displayLog.durationMs != null && (
                    <div>
                      <span className="text-muted-foreground text-xs block mb-1">Latency</span>
                      <span className="font-medium">{formatLatency(displayLog.durationMs)}</span>
                    </div>
                  )}
                  {displayLog.totalTokens != null && (
                    <div>
                      <span className="text-muted-foreground text-xs block mb-1">Tokens Usage</span>
                      <span className="font-medium">
                        {formatTokenCount(displayLog.inputTokens)} in / {formatTokenCount(displayLog.outputTokens)} out
                      </span>
                      <span className="text-muted-foreground text-xs block">
                        {formatTokenCount(displayLog.totalTokens)} total
                      </span>
                    </div>
                  )}
                  {compression && (
                    <div>
                      <span className="text-muted-foreground text-xs block mb-1">Compact</span>
                      <span className="font-medium">
                        {formatCompressionRatio(compression)} saved
                      </span>
                      <span className="text-muted-foreground text-xs block">
                        ~{formatTokenCount(compression.tokensSavedEstimate)} tokens
                      </span>
                    </div>
                  )}
                  {(displayLog.method || displayLog.path) && (
                    <div className="col-span-2">
                      <span className="text-muted-foreground text-xs block mb-1">Request Path</span>
                      <div className="font-mono text-xs break-all bg-muted p-2 rounded border">
                        {displayLog.method && <span className="font-bold mr-2">{displayLog.method}</span>}
                        {displayLog.path || "—"}
                      </div>
                    </div>
                  )}
                </div>
              </div>

              {/* Error Details */}
              {displayLog.error && (
                <div className="space-y-2">
                  <h3 className="font-semibold text-sm text-rose-600 flex items-center gap-1.5">
                    <AlertTriangle className="h-4 w-4" /> Error Summary
                  </h3>
                  <div className="rounded-lg bg-rose-50 dark:bg-rose-950/30 border border-rose-200 dark:border-rose-800 p-3">
                    <p className="text-xs text-rose-700 dark:text-rose-400 whitespace-pre-wrap break-all">
                      {displayLog.error}
                    </p>
                  </div>
                </div>
              )}

              {/* Message Details */}
              {displayLog.message && !displayLog.path && (
                <div className="space-y-2">
                  <h3 className="font-semibold text-sm">Message</h3>
                  <div className="rounded-lg border p-3 bg-muted/20">
                    <p className="text-sm">{displayLog.message}</p>
                  </div>
                </div>
              )}

              {/* Collapsible JSON Sections */}
              {loadingDetail ? (
                <div className="space-y-3 pt-2">
                  <h3 className="font-semibold text-sm mb-2">Payloads</h3>
                  <Skeleton className="h-[100px] w-full" />
                </div>
              ) : (displayLog.requestBody || displayLog.responseBody || displayLog.metadataJson) && (
                <div className="space-y-3 pt-2">
                  <h3 className="font-semibold text-sm mb-2">Payloads</h3>
                  {(() => {
                    let meta: any = {};
                    if (displayLog.metadataJson && typeof displayLog.metadataJson === "string") {
                      try {
                        meta = JSON.parse(displayLog.metadataJson);
                      } catch {}
                    } else if (displayLog.metadataJson && typeof displayLog.metadataJson === "object") {
                      meta = displayLog.metadataJson;
                    }

                    return (
                      <>
                        <PayloadViewer label="Request Headers" rawContent={meta?.requestHeaders} />
                        <PayloadViewer label="Request Body" rawContent={displayLog.requestBody} />
                        <PayloadViewer label="Response Headers" rawContent={meta?.responseHeaders} />
                        <PayloadViewer label="Response Body" rawContent={displayLog.responseBody} />
                        <PayloadViewer label="Metadata" rawContent={displayLog.metadataJson} />
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
