import { useCallback, useEffect, useRef, useState, useMemo } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  RefreshCw,
  ChevronLeft,
  ChevronRight,
  X,
  FileWarning,
  Clock,
  Zap,
  Filter,
  Activity,
  AlertTriangle,
  Radio,
  Copy,
  Check,
  ChevronDown
} from "lucide-react";
import { toast } from "sonner";
import { goApi } from "@/lib/go-api";
import { StatusBadge, formatDateTime, formatLatency } from "./logs/helpers";
import { PayloadViewer } from "./logs/PayloadViewer";
import { cn } from "@/lib/utils";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Switch } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { Input } from "@/components/ui/input";

import type { LogConnectionSummary, LogEntry, LogFilters } from "@/types/logs";

const SSE_BASE = import.meta.env.VITE_GO_API_URL || "/api";

const DEFAULT_FILTERS: LogFilters = {
  range: "24h",
  connectionId: "all",
  provider: "all",
  level: "all",
  q: "",
};

const RANGE_OPTIONS = [
  { value: "1h", label: "Last 1h" },
  { value: "24h", label: "Last 24h" },
  { value: "7d", label: "Last 7d" },
  { value: "30d", label: "Last 30d" },
];

const PROVIDER_OPTIONS = [
  { value: "all", label: "All Providers" },
  { value: "CLIENT", label: "Client" },
  { value: "KIRO", label: "Kiro" },
  { value: "OPENAI", label: "OpenAI" },
  { value: "ANTHROPIC", label: "Anthropic" },
  { value: "GEMINI", label: "Gemini" },
  { value: "OAI_COMPAT", label: "OAI Compatible" },
  { value: "GLM", label: "Zhipu GLM" },
  { value: "QWEN", label: "Alibaba Qwen" },
  { value: "MINIMAX", label: "MiniMax" },
];

function buildFilterParams(filters: LogFilters): URLSearchParams {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value && value !== "all") params.set(key, value);
  });
  return params;
}



export interface LogsScreenProps {
  initialFilters?: Partial<LogFilters>;
  hiddenFilters?: (keyof LogFilters)[];
  embedded?: boolean;
  allowedProviders?: string[];
}

export default function LogsScreen({ 
  initialFilters, 
  hiddenFilters = [], 
  embedded = false,
  allowedProviders
}: LogsScreenProps = {}) {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [connections, setConnections] = useState<LogConnectionSummary[]>([]);
  const [filters, setFilters] = useState<LogFilters>(() => ({
    ...DEFAULT_FILTERS,
    ...initialFilters
  }));
  const [live, setLive] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [debouncedFilters, setDebouncedFilters] = useState<LogFilters>(() => ({
    ...DEFAULT_FILTERS,
    ...initialFilters,
  }));

  // Sync initial filters if they change from parent
  useEffect(() => {
    if (initialFilters) {
      setFilters((prev) => {
        let hasChanges = false;
        const newFilters = { ...prev };
        Object.keys(initialFilters).forEach((key) => {
          const k = key as keyof LogFilters;
          if (initialFilters[k] !== undefined && initialFilters[k] !== prev[k]) {
            newFilters[k] = initialFilters[k] as any;
            hasChanges = true;
          }
        });
        return hasChanges ? newFilters : prev;
      });
    }
  }, [
    initialFilters?.range,
    initialFilters?.connectionId,
    initialFilters?.provider,
    initialFilters?.level,
    initialFilters?.q,
  ]);

  useEffect(() => {
    if (filters.q === debouncedFilters.q) {
      setDebouncedFilters(filters);
      return;
    }

    const timer = setTimeout(() => {
      setDebouncedFilters(filters);
    }, 300);

    return () => clearTimeout(timer);
  }, [filters, debouncedFilters.q]);

  // Client-side pagination config
  const [page, setPage] = useState(1);
  const limit = 20;

  // Detail panel
  const [selectedLogId, setSelectedLogId] = useState<string | null>(null);

  // SSE refs
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const backoffRef = useRef(1000);
  const fetchReqIdRef = useRef(0);
  const logIdSetRef = useRef<Set<string>>(new Set());

  const filteredProviderOptions = useMemo(
    () => PROVIDER_OPTIONS.filter((o) => !allowedProviders || o.value === "all" || o.value === "CLIENT" || allowedProviders.includes(o.value)),
    [allowedProviders],
  );

  const fetchConnections = useCallback(async () => {
    try {
      const res = await goApi.getLogConnections({ range: debouncedFilters.range });
      if (res) setConnections(res);
    } catch {
      // Silently fail
    }
  }, [debouncedFilters.range]);

  const fetchLogs = useCallback(async (isAuto = false) => {
    const skipLoading = Boolean(isAuto);
    const reqId = ++fetchReqIdRef.current;
    if (!skipLoading) setIsLoading(true);

    try {
      const res = await goApi.getLogs(debouncedFilters);

      if (reqId !== fetchReqIdRef.current) return;

      if (Array.isArray(res)) {
        logIdSetRef.current = new Set(res.map((log) => log.id));
        setLogs(res);
        setPage(1); // Reset page on new data
      }
    } catch {
      if (reqId !== fetchReqIdRef.current) return;
      toast.error("Failed to fetch logs");
    } finally {
      if (reqId !== fetchReqIdRef.current) return;
      setIsLoading(false);
      setIsRefreshing(false);
    }
  }, [debouncedFilters]);

  // Initial fetch and when filters (except live) change
  useEffect(() => {
    fetchConnections();
    fetchLogs();
  }, [debouncedFilters, fetchConnections, fetchLogs]);

  // SSE Live stream handling
  useEffect(() => {
    if (!live) {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      return;
    }

    const connectSSE = () => {
      if (eventSourceRef.current) eventSourceRef.current.close();

      const params = buildFilterParams(debouncedFilters);
      // Ensure range is 1h for stream to avoid pulling massive history if they select 30d
      params.set("range", "1h");
      const url = `${SSE_BASE}/logs/stream?${params.toString()}`;

      const sse = new EventSource(url);
      eventSourceRef.current = sse;

      sse.onopen = () => {
        backoffRef.current = 1000;
        setIsLoading(false);
      };

      sse.onmessage = (event) => {
        try {
          const rawData = event.data;
          if (rawData === ": keepalive") return;

          const data = JSON.parse(rawData);
          if (data.type === "init" && Array.isArray(data.logs)) {
            logIdSetRef.current = new Set(data.logs.map((log: LogEntry) => log.id));
            setLogs(data.logs);
            setPage(1);
          } else if (data.type === "delta" && data.log) {
            setLogs((prev) => {
              if (logIdSetRef.current.has(data.log.id)) return prev;
              const newLogs = [data.log, ...prev];
              // Optional: cap array size in live mode
              const cappedLogs = newLogs.slice(0, 1000);
              logIdSetRef.current = new Set(cappedLogs.map((log) => log.id));
              return cappedLogs;
            });
          }
        } catch (e) {
          console.error("SSE parse error", e);
        }
      };

      sse.onerror = (e) => {
        sse.close();
        if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = setTimeout(() => {
          backoffRef.current = Math.min(backoffRef.current * 2, 30000);
          connectSSE();
        }, backoffRef.current);
      };
    };

    connectSSE();

    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
    };
  }, [live, debouncedFilters]);

  const handleRefresh = () => {
    if (live) return;
    setIsRefreshing(true);
    fetchLogs();
  };

  const clearFilters = () => {
    const resetFilters = { ...DEFAULT_FILTERS, ...initialFilters };
    setFilters(resetFilters);
    setDebouncedFilters(resetFilters);
  };

  const hasActiveFilters =
    filters.provider !== (initialFilters?.provider || "all") ||
    filters.level !== (initialFilters?.level || "all") ||
    filters.connectionId !== (initialFilters?.connectionId || "all") ||
    filters.q !== (initialFilters?.q || "") ||
    filters.range !== (initialFilters?.range || "24h");

  const closeDetail = () => setSelectedLogId(null);

  // Pagination logic
  const total = logs.length;
  const totalPages = Math.max(1, Math.ceil(total / limit));
  const startIdx = (page - 1) * limit;
  const endIdx = startIdx + limit;
  const displayedLogs = logs.slice(startIdx, endIdx);

  const selectedLog = useMemo(
    () => logs.find((l) => l.id === selectedLogId),
    [logs, selectedLogId]
  );

  return (
    <div className={cn("flex flex-col h-full", embedded ? "gap-3" : "gap-4 p-4 md:p-6")}>
      {/* Header */}
      {!embedded && (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-2">
            <FileWarning className="h-5 w-5 text-amber-600" />
            <h1 className="text-lg font-semibold">Request Logs</h1>
            {total > 0 && (
              <Badge variant="secondary" className="ml-1">
                {total} total
              </Badge>
            )}
          </div>
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-md border bg-card">
              <Radio
                className={cn(
                  "h-4 w-4",
                  live ? "text-green-500 animate-pulse" : "text-muted-foreground"
                )}
              />
              <span className="text-sm font-medium">Live</span>
              <Switch checked={live} onCheckedChange={setLive} className="ml-1" />
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={handleRefresh}
              disabled={isRefreshing || live}
            >
              <RefreshCw className={cn("mr-2 h-4 w-4", isRefreshing && "animate-spin")} />
              Refresh
            </Button>
          </div>
        </div>
      )}

      {/* Filter Bar */}
      <div className="flex flex-wrap items-center gap-2 rounded-lg border bg-card p-2 md:p-3 shadow-sm">
        {!embedded && (
          <div className="flex items-center gap-1.5 text-sm text-muted-foreground mr-1">
            <Filter className="h-4 w-4" />
            <span className="hidden sm:inline font-medium">Filters</span>
          </div>
        )}

        {!hiddenFilters.includes("range") && (
          <Select
            value={filters.range}
            onValueChange={(v) => setFilters({ ...filters, range: v })}
          >
            <SelectTrigger className="w-[110px]" size="sm">
              <SelectValue placeholder="Range" />
            </SelectTrigger>
            <SelectContent>
              {RANGE_OPTIONS.map((o) => (
                <SelectItem key={o.value} value={o.value}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}

        {!hiddenFilters.includes("provider") && (
          <Select
            value={filters.provider}
            onValueChange={(v) => setFilters({ ...filters, provider: v })}
          >
            <SelectTrigger className="w-[130px]" size="sm">
              <SelectValue placeholder="Provider" />
            </SelectTrigger>
              <SelectContent>
               {filteredProviderOptions.map((o) => (
                 <SelectItem key={o.value} value={o.value}>
                   {o.label}
                 </SelectItem>
               ))}
             </SelectContent>
          </Select>
        )}

        {!hiddenFilters.includes("connectionId") && (
          <Select
            value={filters.connectionId}
            onValueChange={(v) => setFilters({ ...filters, connectionId: v })}
          >
            <SelectTrigger className="w-[140px]" size="sm">
              <SelectValue placeholder="Connection" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Connections</SelectItem>
              {connections.map((conn) => (
                <SelectItem key={conn.connectionId} value={conn.connectionId}>
                  {conn.connectionName || conn.connectionId}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}

        <div className="relative flex-1 min-w-[150px]">
          <Input
            value={filters.q}
            onChange={(e) => setFilters({ ...filters, q: e.target.value })}
            placeholder={embedded ? "Search logs..." : "Search message, model, id..."}
            className="h-8 text-sm"
          />
        </div>

        <div className="flex items-center gap-2 ml-auto shrink-0">
          {!hiddenFilters.includes("level") && (
            <label className="flex items-center gap-1.5 text-sm cursor-pointer">
              <Switch
                checked={filters.level === "ERROR"}
                onCheckedChange={(checked) => {
                  setFilters({ ...filters, level: checked ? "ERROR" : "all" });
                }}
              />
              <span className="text-xs text-muted-foreground hidden lg:inline">
                Errors only
              </span>
            </label>
          )}

          {hasActiveFilters && (
            <Button variant="ghost" size="sm" onClick={clearFilters} className="h-8 px-2 text-muted-foreground hover:bg-muted/50">
              <X className="mr-1 h-3 w-3" />
              <span className="hidden sm:inline">Clear</span>
            </Button>
          )}
          
          {embedded && (
            <>
              <Separator orientation="vertical" className="h-4 mx-1" />
              <div className="flex items-center gap-1.5 px-2 bg-muted/30 rounded-md border py-1">
                <Radio className={cn("h-3.5 w-3.5", live ? "text-green-500 animate-pulse" : "text-muted-foreground")} />
                <span className="text-xs font-medium hidden sm:inline mr-1">Live</span>
                <Switch checked={live} onCheckedChange={setLive} className="scale-75 origin-right" />
              </div>
              <Button variant="outline" size="icon" onClick={handleRefresh} disabled={isRefreshing || live} className="h-7 w-7">
                <RefreshCw className={cn("h-3.5 w-3.5", isRefreshing && "animate-spin")} />
              </Button>
            </>
          )}
        </div>
      </div>

      {/* Logs Table */}
      <div className="flex-1 rounded-lg border bg-card flex flex-col overflow-hidden min-h-[400px]">
        {isLoading ? (
          <div className="p-4 space-y-3">
            {[...Array(6)].map((_, i) => (
              <Skeleton key={i} className="h-12 w-full" />
            ))}
          </div>
        ) : displayedLogs.length === 0 ? (
          <div
            className="flex-1 flex flex-col items-center justify-center py-16 text-center"
          >
            <Clock className="h-10 w-10 text-muted-foreground/50 mb-3" />
            <p className="text-muted-foreground font-medium">No logs found</p>
            <p className="text-sm text-muted-foreground/70 mt-1">
              {hasActiveFilters
                ? "Try adjusting your filters"
                : "Request logs will appear here when requests are made"}
            </p>
          </div>
        ) : (
          <div className="flex-1 overflow-auto">
            <Table>
              <TableHeader className="sticky top-0 bg-card z-10">
                <TableRow>
                  <TableHead className="w-[140px]">Time</TableHead>
                  <TableHead className="w-[60px]">Method</TableHead>
                  <TableHead className="w-[70px]">Status</TableHead>
                  <TableHead className="w-[90px]">Provider</TableHead>
                  <TableHead className="w-[140px]">Model</TableHead>
                  <TableHead className="w-[140px] hidden lg:table-cell">Connection</TableHead>
                  <TableHead className="min-w-[200px]">Path</TableHead>
                  <TableHead className="w-[80px]">Latency</TableHead>
                  <TableHead className="w-[40px] hidden xl:table-cell"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {displayedLogs.map((log) => (
                  <TableRow
                    key={log.id}
                    className="cursor-pointer hover:bg-muted/50 transition-colors"
                    onClick={() => setSelectedLogId(log.id)}
                  >
                    <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                      {formatDateTime(log.timestamp)}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="font-mono text-[10px]">
                        {log.method || "—"}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <StatusBadge status={log.statusCode} level={log.level} />
                    </TableCell>
                    <TableCell className="text-xs">{log.provider || "—"}</TableCell>
                    <TableCell className="text-xs max-w-[140px] truncate" title={log.model || ""}>
                      {log.model || "—"}
                    </TableCell>
                    <TableCell className="text-xs hidden lg:table-cell max-w-[140px] truncate" title={log.connectionName || ""}>
                      {log.connectionName || "—"}
                    </TableCell>
                    <TableCell className="font-mono text-xs max-w-[200px] truncate" title={log.path || log.message}>
                      {log.path || log.message || "—"}
                    </TableCell>
                    <TableCell className="text-xs">
                      <div className="flex items-center gap-1">
                        <Zap
                          className={cn(
                            "h-3 w-3",
                            log.durationMs && log.durationMs > 3000
                              ? "text-amber-500"
                              : "text-muted-foreground"
                          )}
                        />
                        {formatLatency(log.durationMs)}
                      </div>
                    </TableCell>
                    <TableCell className="hidden xl:table-cell">
                      {log.level === "ERROR" && (
                        <FileWarning className="h-4 w-4 text-rose-500" />
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}

        {/* Pagination Footer */}
        {total > 0 && (
          <>
            <Separator />
            <div className="flex items-center justify-between px-4 py-3 shrink-0 bg-card">
              <p className="text-xs text-muted-foreground">
                Showing {Math.min(startIdx + 1, total)}–{Math.min(endIdx, total)} of {total}
              </p>
              <div className="flex items-center gap-1">
                <Button
                  variant="outline"
                  size="icon"
                  className="h-7 w-7"
                  disabled={page <= 1}
                  onClick={() => setPage(p => p - 1)}
                >
                  <ChevronLeft className="h-3.5 w-3.5" />
                </Button>
                <span className="text-xs px-2 text-muted-foreground">
                  Page {page} of {totalPages}
                </span>
                <Button
                  variant="outline"
                  size="icon"
                  className="h-7 w-7"
                  disabled={page >= totalPages}
                  onClick={() => setPage(p => p + 1)}
                >
                  <ChevronRight className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          </>
        )}
      </div>

      {/* Detail Sheet */}
      <Sheet open={!!selectedLogId} onOpenChange={(open) => !open && closeDetail()}>
        <SheetContent side="right" className="w-full sm:max-w-xl overflow-y-auto p-0">
          {selectedLog ? (
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
                      <span className="font-medium">{formatDateTime(selectedLog.timestamp)}</span>
                    </div>
                    <div>
                      <span className="text-muted-foreground text-xs block mb-1">Status</span>
                      <span className="flex items-center">
                        <StatusBadge status={selectedLog.statusCode} level={selectedLog.level} />
                      </span>
                    </div>
                    <div>
                      <span className="text-muted-foreground text-xs block mb-1">Provider & Model</span>
                      <div className="font-medium truncate" title={selectedLog.model || ""}>
                        {selectedLog.provider || "—"} / {selectedLog.model || "—"}
                      </div>
                    </div>
                    <div>
                      <span className="text-muted-foreground text-xs block mb-1">Connection</span>
                      <span className="font-medium">{selectedLog.connectionName || selectedLog.connectionId || "—"}</span>
                    </div>
                    {selectedLog.durationMs != null && (
                      <div>
                        <span className="text-muted-foreground text-xs block mb-1">Latency</span>
                        <span className="font-medium">{formatLatency(selectedLog.durationMs)}</span>
                      </div>
                    )}
                    {selectedLog.totalTokens != null && (
                      <div>
                        <span className="text-muted-foreground text-xs block mb-1">Tokens Usage</span>
                        <span className="font-medium">{selectedLog.totalTokens.toLocaleString()} Total</span>
                      </div>
                    )}
                    {(selectedLog.method || selectedLog.path) && (
                      <div className="col-span-2">
                        <span className="text-muted-foreground text-xs block mb-1">Request Path</span>
                        <div className="font-mono text-xs break-all bg-muted p-2 rounded border">
                          {selectedLog.method && <span className="font-bold mr-2">{selectedLog.method}</span>}
                          {selectedLog.path || "—"}
                        </div>
                      </div>
                    )}
                  </div>
                </div>

                {/* Error Details */}
                {selectedLog.error && (
                  <div className="space-y-2">
                    <h3 className="font-semibold text-sm text-rose-600 flex items-center gap-1.5">
                      <AlertTriangle className="h-4 w-4" /> Error Summary
                    </h3>
                    <div className="rounded-lg bg-rose-50 dark:bg-rose-950/30 border border-rose-200 dark:border-rose-800 p-3">
                      <p className="text-xs text-rose-700 dark:text-rose-400 whitespace-pre-wrap break-all">
                        {selectedLog.error}
                      </p>
                    </div>
                  </div>
                )}

                {/* Message Details */}
                {selectedLog.message && !selectedLog.path && (
                  <div className="space-y-2">
                    <h3 className="font-semibold text-sm">Message</h3>
                    <div className="rounded-lg border p-3 bg-muted/20">
                      <p className="text-sm">{selectedLog.message}</p>
                    </div>
                  </div>
                )}

                {/* Collapsible JSON Sections */}
                {(selectedLog.requestBody || selectedLog.responseBody || selectedLog.metadataJson) && (
                  <div className="space-y-3 pt-2">
                    <h3 className="font-semibold text-sm mb-2">Payloads</h3>
                    {(() => {
                        let meta: any = {};
                        if (selectedLog.metadataJson && typeof selectedLog.metadataJson === 'string') {
                            try { meta = JSON.parse(selectedLog.metadataJson); } catch {}
                        } else if (selectedLog.metadataJson && typeof selectedLog.metadataJson === 'object') {
                            meta = selectedLog.metadataJson;
                        }
                        
                        return (
                            <>
                                <PayloadViewer label="Request Headers" rawContent={meta?.requestHeaders} />
                                <PayloadViewer label="Request Body" rawContent={selectedLog.requestBody} />
                                <PayloadViewer label="Response Headers" rawContent={meta?.responseHeaders} />
                                <PayloadViewer label="Response Body" rawContent={selectedLog.responseBody} />
                                <PayloadViewer label="Metadata" rawContent={selectedLog.metadataJson} />
                            </>
                        )
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
    </div>
  );
}
