import { useCallback, useEffect, useRef, useState, useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import { FileWarning, RefreshCw, Radio, Terminal, Table2, Eye } from "lucide-react";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import { goApi, consumeSSE } from "@/lib/go-api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import type { LogConnectionSummary, LogEntry, LogFilters } from "@/types/logs";
import { FilterBar } from "./logs/filter-bar";
import { LogsTable } from "./logs/logs-table";
import { LogDetailSheet } from "./logs/log-detail-sheet";
import { ConsoleViewer } from "./logs/console-viewer";
import { LiveFeed } from "./dashboard/live-feed";
import { buildFilterParams, DEFAULT_FILTERS } from "./logs/helpers";

type ViewTab = "table" | "console" | "live";

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
  allowedProviders,
}: LogsScreenProps = {}) {
  const [searchParams] = useSearchParams();
  const queryProvider = searchParams.get("provider") || undefined;
  const queryConnectionId = searchParams.get("connectionId") || undefined;
  const urlInitialFilters = useMemo(
    () => ({
      ...(queryProvider ? { provider: queryProvider } : {}),
      ...(queryConnectionId ? { connectionId: queryConnectionId } : {}),
    }),
    [queryProvider, queryConnectionId]
  );
  const [viewTab, setViewTab] = useState<ViewTab>("table");
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [connections, setConnections] = useState<LogConnectionSummary[]>([]);
  const [filters, setFilters] = useState<LogFilters>(() => ({
    ...DEFAULT_FILTERS,
    ...initialFilters,
    ...urlInitialFilters,
  }));
  const [live, setLive] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [debouncedFilters, setDebouncedFilters] = useState<LogFilters>(() => ({
    ...DEFAULT_FILTERS,
    ...initialFilters,
    ...urlInitialFilters,
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
    queryProvider,
    queryConnectionId,
  ]);

  // Apply provider / connectionId filters from the URL on navigation
  useEffect(() => {
    setFilters((prev) => {
      const next = { ...prev };
      let changed = false;
      if (queryProvider !== undefined && queryProvider !== prev.provider) {
        next.provider = queryProvider;
        changed = true;
      }
      if (queryConnectionId !== undefined && queryConnectionId !== prev.connectionId) {
        next.connectionId = queryConnectionId;
        changed = true;
      }
      return changed ? next : prev;
    });
  }, [queryProvider, queryConnectionId]);

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
  const sseAbortRef = useRef<AbortController | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const backoffRef = useRef(1000);
  const fetchReqIdRef = useRef(0);
  const logIdSetRef = useRef<Set<string>>(new Set());

  const fetchConnections = useCallback(async () => {
    try {
      const res = await goApi.getLogConnections({ range: debouncedFilters.range });
      if (res) setConnections(res);
    } catch {
      // Silently fail
    }
  }, [debouncedFilters.range]);

  const fetchLogs = useCallback(
    async (isAuto = false) => {
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
    },
    [debouncedFilters]
  );

  // Initial fetch and when filters (except live) change
  useEffect(() => {
    fetchConnections();
    fetchLogs();
  }, [debouncedFilters, fetchConnections, fetchLogs]);

  // SSE Live stream handling
  useEffect(() => {
    if (!live) {
      sseAbortRef.current?.abort();
      sseAbortRef.current = null;
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      return;
    }

    const handlePayload = (rawData: string) => {
      if (!rawData || rawData === ": keepalive") return;
      try {
        const data = JSON.parse(rawData);
        if (data.type === "init" && Array.isArray(data.logs)) {
          logIdSetRef.current = new Set(data.logs.map((log: LogEntry) => log.id));
          setLogs(data.logs);
          setPage(1);
        } else if (data.type === "delta" && data.log) {
          setLogs((prev) => {
            if (logIdSetRef.current.has(data.log.id)) return prev;
            const newLogs = [data.log, ...prev];
            const cappedLogs = newLogs.slice(0, 1000);
            logIdSetRef.current = new Set(cappedLogs.map((log) => log.id));
            return cappedLogs;
          });
        }
      } catch (e) {
        console.error("SSE parse error", e);
      }
    };

    const connectSSE = () => {
      sseAbortRef.current?.abort();
      const controller = new AbortController();
      sseAbortRef.current = controller;
      const params = buildFilterParams(debouncedFilters);
      void consumeSSE(`/api/logs/stream?${params.toString()}`, handlePayload, controller.signal)
        .then(() => {
          backoffRef.current = 1000;
          setIsLoading(false);
        })
        .catch((err) => {
          if (controller.signal.aborted) return;
          if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
          reconnectTimerRef.current = setTimeout(() => {
            backoffRef.current = Math.min(backoffRef.current * 2, 30000);
            connectSSE();
          }, backoffRef.current);
          console.error(err);
        });
      backoffRef.current = 1000;
      setIsLoading(false);
    };

    connectSSE();

    return () => {
      sseAbortRef.current?.abort();
      sseAbortRef.current = null;
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

  const selectedLog = useMemo(
    () => logs.find((l) => l.id === selectedLogId),
    [logs, selectedLogId]
  );

  const isConsole = !embedded && viewTab === "console";
  const isLive = !embedded && viewTab === "live";

  return (
    <div className={cn("flex flex-col h-full", embedded ? "gap-3" : "gap-4 p-4 md:p-6")}>
      {/* Header */}
      {!embedded && (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-2">
            <FileWarning className="h-5 w-5 text-amber-600" />
            <h1 className="text-lg font-semibold">Request Logs</h1>
            {viewTab === "table" && logs.length > 0 && (
              <Badge variant="secondary" className="ml-1">
                {logs.length} total
              </Badge>
            )}
          </div>
          <div className="flex items-center gap-3">
            {/* Tab Switcher */}
            <div className="flex items-center rounded-md border bg-card p-0.5">
              <button
                onClick={() => setViewTab("table")}
                className={cn(
                  "flex items-center gap-1.5 px-2.5 py-1 rounded text-sm font-medium transition-colors",
                  viewTab === "table"
                    ? "bg-primary text-primary-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                <Table2 className="h-3.5 w-3.5" />
                Table
              </button>
              <button
                onClick={() => setViewTab("console")}
                className={cn(
                  "flex items-center gap-1.5 px-2.5 py-1 rounded text-sm font-medium transition-colors",
                  viewTab === "console"
                    ? "bg-primary text-primary-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                <Terminal className="h-3.5 w-3.5" />
                Console
              </button>
              <button
                onClick={() => setViewTab("live")}
                className={cn(
                  "flex items-center gap-1.5 px-2.5 py-1 rounded text-sm font-medium transition-colors",
                  viewTab === "live"
                    ? "bg-primary text-primary-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                <Eye className="h-3.5 w-3.5" />
                Live
              </button>
            </div>

            {/* Live toggle + Refresh — only for table view */}
            {viewTab === "table" && (
              <>
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
              </>
            )}
          </div>
        </div>
      )}

      {/* Console View */}
      {isConsole && (
        <div className="flex-1 min-h-0 relative">
          <ConsoleViewer filters={debouncedFilters} />
        </div>
      )}

      {/* Live Feed View */}
      {isLive && (
        <div className="flex-1 min-h-0">
          <LiveFeed />
        </div>
      )}

      {/* Table View (also used for embedded mode) */}
      {!isConsole && !isLive && (
        <>
          {/* Filter Bar */}
          <FilterBar
            filters={filters}
            onFiltersChange={setFilters}
            connections={connections}
            hiddenFilters={hiddenFilters}
            embedded={embedded}
            allowedProviders={allowedProviders}
            live={live}
            onLiveChange={setLive}
            onRefresh={handleRefresh}
            isRefreshing={isRefreshing}
            hasActiveFilters={hasActiveFilters}
            onClearFilters={clearFilters}
          />

          {/* Logs Table */}
          <LogsTable
            logs={logs}
            isLoading={isLoading}
            page={page}
            limit={limit}
            onPageChange={setPage}
            onLogSelect={setSelectedLogId}
            hasActiveFilters={hasActiveFilters}
          />

          {/* Detail Sheet */}
          <LogDetailSheet
            log={selectedLog}
            open={!!selectedLogId}
            onOpenChange={(open) => !open && closeDetail()}
          />
        </>
      )}
    </div>
  );
}
