import { useCallback, useEffect, useRef, useState } from "react";
import {
  Activity,
  AlertTriangle,
  ChevronDown,
  ChevronRight,
  Clock,
  DollarSign,
  FileText,
  Radio,
  RefreshCw,
  Search,
  Server,
  Trash2,
  Zap,
} from "lucide-react";
import { toast } from "sonner";

import { goApi } from "@/lib/go-api";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

import type {
  LogConnectionSummary,
  LogEntry,
  LogFilters,
  LogSummary,
} from "@/types/logs";

// ─── Constants ───────────────────────────────────────────────────────────────

const SSE_BASE = import.meta.env.VITE_GO_API_URL || "/api";

const DEFAULT_FILTERS: LogFilters = {
  range: "24h",
  connectionId: "all",
  provider: "all",
  level: "all",
  q: "",
};

const RANGE_OPTIONS = [
  { value: "1h", label: "Last hour" },
  { value: "24h", label: "Last 24 hours" },
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
];

const PROVIDER_OPTIONS = [
  { value: "all", label: "All providers" },
  { value: "CLIENT", label: "Client" },
  { value: "KIRO", label: "Kiro" },
  { value: "OPENAI", label: "OpenAI" },
];

const LEVEL_OPTIONS = [
  { value: "all", label: "All levels" },
  { value: "INFO", label: "Info" },
  { value: "ERROR", label: "Errors" },
];

// ─── Helpers ─────────────────────────────────────────────────────────────────

function formatNumber(value = 0): string {
  return new Intl.NumberFormat().format(value);
}

function formatCost(value = 0, currency = "USD"): string {
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency,
    maximumFractionDigits: 4,
  }).format(value);
}

function formatTime(value?: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function formatDateTime(value?: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

interface LogMetadata {
  responsePreview?: string;
  truncated?: boolean;
  source?: string;
}

function parseMetadata(value?: string): LogMetadata {
  if (!value) return {};
  try {
    const parsed = JSON.parse(value) as Record<string, unknown>;
    return {
      responsePreview:
        typeof parsed.responsePreview === "string"
          ? parsed.responsePreview
          : undefined,
      truncated:
        typeof parsed.truncated === "boolean" ? parsed.truncated : false,
      source: typeof parsed.source === "string" ? parsed.source : undefined,
    };
  } catch {
    return {};
  }
}

function prettyJSON(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

function directionBarColor(direction?: string): string {
  switch (direction) {
    case "inbound":
      return "bg-blue-500";
    case "outbound":
      return "bg-purple-500";
    case "response":
      return "bg-green-500";
    case "usage":
      return "bg-amber-500";
    case "payload":
      return "bg-emerald-500";
    default:
      return "bg-muted-foreground/40";
  }
}

function directionTitle(log: LogEntry): string {
  switch (log.direction) {
    case "inbound":
      return "Client request";
    case "outbound":
      return "Provider request";
    case "response":
      return log.level === "ERROR"
        ? "Provider error response"
        : "Provider response";
    case "usage":
      return "Usage captured";
    case "payload":
      return "Response payload";
    default:
      return "System event";
  }
}

function statusVariant(
  statusCode?: number,
  level?: string,
): "default" | "secondary" | "destructive" | "outline" {
  if (level === "ERROR" || (statusCode && statusCode >= 400))
    return "destructive";
  if (statusCode && statusCode >= 200 && statusCode < 300) return "default";
  return "secondary";
}

function previewText(log: LogEntry, metadata: LogMetadata): string {
  if (metadata.responsePreview) return metadata.responsePreview;
  if (log.error) return log.error;
  if (log.direction === "usage") {
    const inp = log.inputTokens || 0;
    const out = log.outputTokens || 0;
    return `${inp.toLocaleString()} input + ${out.toLocaleString()} output tokens`;
  }
  return log.message || "";
}

function buildFilterParams(filters: LogFilters): URLSearchParams {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value && value !== "all") params.set(key, value);
  });
  return params;
}

// ─── Summary Bar ─────────────────────────────────────────────────────────────

function SummaryBar({ summary }: { summary: LogSummary | null }) {
  const stats = [
    {
      label: "Requests",
      value: formatNumber(summary?.requests),
      icon: Activity,
      color: "text-blue-500",
      bg: "bg-blue-500/10",
    },
    {
      label: "Errors",
      value: formatNumber(summary?.errors),
      icon: AlertTriangle,
      color: "text-destructive",
      bg: "bg-destructive/10",
    },
    {
      label: "Tokens",
      value: formatNumber(summary?.totalTokens),
      icon: Zap,
      color: "text-green-500",
      bg: "bg-green-500/10",
    },
    {
      label: "Est. Cost",
      value: formatCost(summary?.costTotal, summary?.currency || "USD"),
      icon: DollarSign,
      color: "text-amber-500",
      bg: "bg-amber-500/10",
    },
  ];

  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      {stats.map((stat) => (
        <div
          key={stat.label}
          className="rounded-lg border bg-card p-3 shadow-sm"
        >
          <div className="flex items-center gap-2">
            <div className={cn("rounded-md p-1.5", stat.bg)}>
              <stat.icon className={cn("size-3.5", stat.color)} />
            </div>
            <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
              {stat.label}
            </span>
          </div>
          <div className="mt-2 text-xl font-bold tracking-tight">
            {stat.value}
          </div>
        </div>
      ))}
    </div>
  );
}

// ─── Filter Bar ──────────────────────────────────────────────────────────────

function FilterBar({
  filters,
  live,
  onFiltersChange,
  onLiveChange,
  onRefresh,
  onClear,
  refreshing,
}: {
  filters: LogFilters;
  live: boolean;
  onFiltersChange: (filters: LogFilters) => void;
  onLiveChange: (live: boolean) => void;
  onRefresh: () => void;
  onClear: () => void;
  refreshing: boolean;
}) {
  const setFilter = (key: keyof LogFilters, value: string) => {
    onFiltersChange({ ...filters, [key]: value });
  };

  return (
    <div className="flex flex-col gap-2 rounded-lg border bg-card p-3 shadow-sm lg:flex-row lg:items-center">
      <Select
        value={filters.range}
        onValueChange={(v) => setFilter("range", v)}
      >
        <SelectTrigger className="w-full lg:w-[150px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {RANGE_OPTIONS.map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select
        value={filters.provider}
        onValueChange={(v) => setFilter("provider", v)}
      >
        <SelectTrigger className="w-full lg:w-[150px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {PROVIDER_OPTIONS.map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select
        value={filters.level}
        onValueChange={(v) => setFilter("level", v)}
      >
        <SelectTrigger className="w-full lg:w-[130px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {LEVEL_OPTIONS.map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <div className="relative flex-1 min-w-0">
        <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={filters.q}
          onChange={(e) => setFilter("q", e.target.value)}
          placeholder="Search model, request id, message…"
          className="pl-9"
        />
      </div>

      <div className="flex items-center gap-2 shrink-0">
        <div className="flex items-center gap-1.5 rounded-md border px-2.5 py-1.5">
          <Radio
            className={cn(
              "size-3.5",
              live ? "text-green-500 animate-pulse" : "text-muted-foreground",
            )}
          />
          <span className="text-xs font-medium">Live</span>
          <Switch checked={live} onCheckedChange={onLiveChange} />
        </div>

        <Button
          variant="outline"
          size="icon"
          onClick={onRefresh}
          disabled={refreshing}
          title="Refresh"
        >
          <RefreshCw className={cn("size-4", refreshing && "animate-spin")} />
        </Button>

        <Button
          variant="outline"
          size="icon"
          onClick={onClear}
          title="Clear logs"
          className="text-destructive hover:text-destructive hover:bg-destructive/10"
        >
          <Trash2 className="size-4" />
        </Button>
      </div>
    </div>
  );
}

// ─── Connection Sidebar ──────────────────────────────────────────────────────

function ConnectionSidebar({
  connections,
  selectedId,
  onSelect,
  collapsed,
  onToggleCollapse,
}: {
  connections: LogConnectionSummary[];
  selectedId: string;
  onSelect: (id: string) => void;
  collapsed: boolean;
  onToggleCollapse: () => void;
}) {
  const safe = Array.isArray(connections) ? connections : [];

  return (
    <aside className="shrink-0 lg:w-64">
      {/* Mobile toggle */}
      <button
        onClick={onToggleCollapse}
        className="flex w-full items-center justify-between rounded-lg border bg-card p-3 text-sm font-medium shadow-sm lg:hidden"
      >
        <div className="flex items-center gap-2">
          <Server className="size-4 text-muted-foreground" />
          Connections
          <Badge variant="secondary" className="text-[10px]">
            {safe.length}
          </Badge>
        </div>
        {collapsed ? (
          <ChevronRight className="size-4 text-muted-foreground" />
        ) : (
          <ChevronDown className="size-4 text-muted-foreground" />
        )}
      </button>

      <div
        className={cn(
          "mt-2 overflow-hidden rounded-lg border bg-card shadow-sm lg:mt-0 lg:block",
          collapsed && "hidden lg:block",
        )}
      >
        <button
          onClick={() => onSelect("all")}
          className={cn(
            "w-full border-b px-4 py-3 text-left transition-colors",
            selectedId === "all"
              ? "bg-accent text-accent-foreground"
              : "hover:bg-muted/50",
          )}
        >
          <div className="text-sm font-semibold">All connections</div>
          <div className="mt-0.5 text-[11px] text-muted-foreground">
            {safe.length} active in range
          </div>
        </button>

        <ScrollArea className="max-h-[480px]">
          {safe.length === 0 ? (
            <div className="p-4 text-xs text-muted-foreground">
              No connection activity in this range.
            </div>
          ) : (
            safe.map((conn) => (
              <button
                key={conn.connectionId}
                onClick={() => onSelect(conn.connectionId)}
                className={cn(
                  "w-full border-b px-4 py-3 text-left transition-colors",
                  selectedId === conn.connectionId
                    ? "bg-accent text-accent-foreground"
                    : "hover:bg-muted/50",
                )}
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate text-sm font-medium">
                    {conn.connectionName || conn.connectionId}
                  </span>
                  {conn.errors > 0 && (
                    <Badge variant="destructive" className="text-[10px]">
                      {conn.errors} err
                    </Badge>
                  )}
                </div>
                <div className="mt-0.5 text-[11px] text-muted-foreground">
                  {conn.provider} &middot; {conn.requests} req
                </div>
                <div className="mt-1.5 flex items-center justify-between gap-2 text-[11px] text-muted-foreground">
                  <span className="font-mono">
                    {conn.totalTokens.toLocaleString()} tok
                  </span>
                  <span className="font-mono">
                    {formatCost(conn.costTotal, conn.currency || "USD")}
                  </span>
                </div>
              </button>
            ))
          )}
        </ScrollArea>
      </div>
    </aside>
  );
}

// ─── Log Event Card ──────────────────────────────────────────────────────────

function LogEventCard({ log }: { log: LogEntry }) {
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [previewOpen, setPreviewOpen] = useState(false);
  const [reqBodyOpen, setReqBodyOpen] = useState(false);
  const [resBodyOpen, setResBodyOpen] = useState(false);

  const metadata = parseMetadata(log.metadataJson);
  const preview = previewText(log, metadata);

  const metricChips: { label: string; icon: typeof Clock }[] = [];
  if (log.durationMs)
    metricChips.push({ label: `${log.durationMs}ms`, icon: Clock });
  if (log.bodySize)
    metricChips.push({
      label: `${log.bodySize.toLocaleString()} B`,
      icon: FileText,
    });
  if (log.totalTokens)
    metricChips.push({
      label: `${log.totalTokens.toLocaleString()} tok`,
      icon: Zap,
    });
  if (log.costTotal)
    metricChips.push({
      label: formatCost(log.costTotal, log.currency || "USD"),
      icon: DollarSign,
    });

  const hasPreview = Boolean(preview);
  const hasDetails =
    log.requestId || log.path || log.message || log.usageSource || log.error;
  const hasReqBody = Boolean(log.requestBody);
  const hasResBody = Boolean(log.responseBody);

  return (
    <article
      className={cn(
        "group relative overflow-hidden rounded-lg border bg-card shadow-sm transition-shadow hover:shadow-md",
        log.level === "ERROR" && "border-destructive/30",
      )}
    >
      {/* Direction color bar */}
      <div
        className={cn(
          "absolute left-0 top-0 bottom-0 w-[3px]",
          directionBarColor(log.direction),
        )}
      />

      <div className="p-3 pl-4">
        {/* Header row */}
        <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="text-sm font-semibold">
                {directionTitle(log)}
              </span>
              <span className="font-mono text-[11px] text-muted-foreground">
                {formatTime(log.timestamp)}
              </span>
              <Badge
                variant={statusVariant(log.statusCode, log.level)}
                className="text-[10px]"
              >
                {log.statusCode || log.level}
              </Badge>
              <Badge variant="outline" className="text-[10px]">
                {log.provider}
                {log.direction ? ` / ${log.direction}` : ""}
              </Badge>
            </div>

            {/* Connection & model line */}
            <div className="mt-1.5 flex flex-wrap gap-x-4 gap-y-0.5 text-[11px] text-muted-foreground">
              {(log.connectionName || log.connectionId) && (
                <span className="truncate">
                  <span className="font-medium text-foreground/70">Conn:</span>{" "}
                  {log.connectionName || log.connectionId}
                </span>
              )}
              {log.model && (
                <span className="truncate">
                  <span className="font-medium text-foreground/70">Model:</span>{" "}
                  {log.model}
                </span>
              )}
            </div>
          </div>

          {/* Metric chips */}
          {metricChips.length > 0 && (
            <div className="flex flex-wrap gap-1 sm:justify-end shrink-0">
              {metricChips.map((chip) => (
                <Badge
                  key={chip.label}
                  variant="secondary"
                  className="gap-1 font-mono text-[10px]"
                >
                  <chip.icon className="size-3" />
                  {chip.label}
                </Badge>
              ))}
            </div>
          )}
        </div>

        {/* Preview section */}
        {hasPreview && (
          <div className="mt-2.5">
            <button
              onClick={() => setPreviewOpen(!previewOpen)}
              className="flex items-center gap-1 text-[11px] font-medium text-muted-foreground hover:text-foreground transition-colors"
            >
              {previewOpen ? (
                <ChevronDown className="size-3" />
              ) : (
                <ChevronRight className="size-3" />
              )}
              {metadata.responsePreview
                ? "Response preview"
                : log.error
                  ? "Error"
                  : "Summary"}
              {metadata.truncated && (
                <span className="text-[10px] italic text-muted-foreground/70">
                  (truncated)
                </span>
              )}
            </button>
            {previewOpen && (
              <pre className="mt-1.5 max-h-44 overflow-auto rounded-md border bg-muted/50 p-3 text-xs leading-relaxed whitespace-pre-wrap break-words text-muted-foreground">
                {preview}
              </pre>
            )}
          </div>
        )}

        {/* Details section */}
        {hasDetails && (
          <div className="mt-2">
            <button
              onClick={() => setDetailsOpen(!detailsOpen)}
              className="flex items-center gap-1 text-[11px] font-medium text-muted-foreground hover:text-foreground transition-colors"
            >
              {detailsOpen ? (
                <ChevronDown className="size-3" />
              ) : (
                <ChevronRight className="size-3" />
              )}
              Details
            </button>
            {detailsOpen && (
              <dl className="mt-1.5 grid gap-1.5 rounded-md border bg-muted/50 p-3 text-[11px] sm:grid-cols-2">
                {log.requestId && (
                  <div className="truncate">
                    <dt className="inline font-medium text-foreground/70">
                      Request ID:
                    </dt>{" "}
                    <dd className="inline font-mono text-muted-foreground">
                      {log.requestId}
                    </dd>
                  </div>
                )}
                {log.path && (
                  <div className="truncate">
                    <dt className="inline font-medium text-foreground/70">
                      Path:
                    </dt>{" "}
                    <dd className="inline font-mono text-muted-foreground">
                      {log.method ? `${log.method} ` : ""}
                      {log.path}
                    </dd>
                  </div>
                )}
                {log.message && (
                  <div className="sm:col-span-2">
                    <dt className="inline font-medium text-foreground/70">
                      Message:
                    </dt>{" "}
                    <dd className="inline text-muted-foreground">
                      {log.message}
                    </dd>
                  </div>
                )}
                {(log.usageSource || metadata.source) && (
                  <div>
                    <dt className="inline font-medium text-foreground/70">
                      Usage source:
                    </dt>{" "}
                    <dd className="inline text-muted-foreground">
                      {log.usageSource || metadata.source}
                    </dd>
                  </div>
                )}
                {log.inputTokens != null && (
                  <div>
                    <dt className="inline font-medium text-foreground/70">
                      Tokens:
                    </dt>{" "}
                    <dd className="inline font-mono text-muted-foreground">
                      {(log.inputTokens || 0).toLocaleString()} in /{" "}
                      {(log.outputTokens || 0).toLocaleString()} out
                    </dd>
                  </div>
                )}
                {log.error && (
                  <div className="sm:col-span-2 text-destructive">
                    <dt className="inline font-medium">Error:</dt>{" "}
                    <dd className="inline">{log.error}</dd>
                  </div>
                )}
              </dl>
            )}
          </div>
        )}

        {/* Request / Response bodies */}
        {(hasReqBody || hasResBody) && (
          <div className="mt-2 flex flex-col gap-1.5">
            {hasReqBody && (
              <div>
                <button
                  onClick={() => setReqBodyOpen(!reqBodyOpen)}
                  className="flex items-center gap-1 text-[11px] font-medium text-muted-foreground hover:text-foreground transition-colors"
                >
                  {reqBodyOpen ? (
                    <ChevronDown className="size-3" />
                  ) : (
                    <ChevronRight className="size-3" />
                  )}
                  Request Body
                </button>
                {reqBodyOpen && (
                  <pre className="mt-1.5 max-h-96 overflow-auto rounded-md border bg-muted/50 p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap break-all text-muted-foreground">
                    {prettyJSON(log.requestBody!)}
                  </pre>
                )}
              </div>
            )}
            {hasResBody && (
              <div>
                <button
                  onClick={() => setResBodyOpen(!resBodyOpen)}
                  className="flex items-center gap-1 text-[11px] font-medium text-muted-foreground hover:text-foreground transition-colors"
                >
                  {resBodyOpen ? (
                    <ChevronDown className="size-3" />
                  ) : (
                    <ChevronRight className="size-3" />
                  )}
                  Response Body
                </button>
                {resBodyOpen && (
                  <pre className="mt-1.5 max-h-96 overflow-auto rounded-md border bg-muted/50 p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap break-all text-muted-foreground">
                    {prettyJSON(log.responseBody!)}
                  </pre>
                )}
              </div>
            )}
          </div>
        )}
      </div>
    </article>
  );
}

// ─── Empty State ─────────────────────────────────────────────────────────────

function EmptyState() {
  return (
    <div className="flex h-64 flex-col items-center justify-center rounded-lg border border-dashed bg-card text-center">
      <FileText className="mb-3 size-10 text-muted-foreground/50" />
      <p className="text-sm font-medium text-muted-foreground">No logs found</p>
      <p className="mt-1 text-xs text-muted-foreground/70">
        Try adjusting your filters or wait for new requests.
      </p>
    </div>
  );
}

// ─── Main Screen ─────────────────────────────────────────────────────────────

export default function LogsScreen() {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [summary, setSummary] = useState<LogSummary | null>(null);
  const [connections, setConnections] = useState<LogConnectionSummary[]>([]);
  const [filters, setFilters] = useState<LogFilters>(DEFAULT_FILTERS);
  const [live, setLive] = useState(false);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [clearDialogOpen, setClearDialogOpen] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(true);

  // SSE refs
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const backoffRef = useRef(1000);

  // ── Data fetching ────────────────────────────────────────────────────────

  const fetchAll = useCallback(
    async (opts?: { showLoading?: boolean }) => {
      if (opts?.showLoading) setLoading(true);
      try {
        const [logData, summaryData, connectionData] = await Promise.all([
          goApi.getLogs({ ...filters, limit: 200 }),
          goApi.getLogSummary(filters),
          goApi.getLogConnections({ range: filters.range }),
        ]);
        setLogs(Array.isArray(logData) ? logData : []);
        setSummary(summaryData ?? null);
        setConnections(Array.isArray(connectionData) ? connectionData : []);
      } catch {
        toast.error("Failed to load logs");
      } finally {
        setLoading(false);
      }
    },
    [filters],
  );

  const handleRefresh = useCallback(async () => {
    setRefreshing(true);
    try {
      const [logData, summaryData, connectionData] = await Promise.all([
        goApi.getLogs({ ...filters, limit: 200 }),
        goApi.getLogSummary(filters),
        goApi.getLogConnections({ range: filters.range }),
      ]);
      setLogs(Array.isArray(logData) ? logData : []);
      setSummary(summaryData ?? null);
      setConnections(Array.isArray(connectionData) ? connectionData : []);
    } catch {
      toast.error("Failed to refresh logs");
    } finally {
      setRefreshing(false);
    }
  }, [filters]);

  // ── Initial load + reload on filter change ───────────────────────────────

  useEffect(() => {
    fetchAll({ showLoading: true });
  }, [fetchAll]);

  // ── SSE streaming ────────────────────────────────────────────────────────

  const cleanupSSE = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  }, []);

  const connectSSE = useCallback(() => {
    cleanupSSE();
    backoffRef.current = 1000;

    const params = buildFilterParams(filters);
    const url = `${SSE_BASE}/logs/stream?${params.toString()}`;
    const es = new EventSource(url);

    es.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data);
        if (Array.isArray(payload)) {
          // Legacy generic fallback
          setLogs(payload);
        } else if (payload.type === "init") {
          setLogs(payload.logs || []);
        } else if (payload.type === "delta" && payload.log) {
          setLogs((prev) => {
            const newLog = payload.log;
            const existsIndex = prev.findIndex((l) => l.id === newLog.id);
            if (existsIndex >= 0) {
              const next = [...prev];
              next[existsIndex] = newLog;
              return next;
            }
            // prepend new logs since list is descending chronologically
            return [newLog, ...prev].slice(0, 500); // optional cap
          });
        }
      } catch {
        // ignore parse errors
      }

      // Refresh summary + connections in background
      goApi
        .getLogSummary(filters)
        .then((data: LogSummary | null) => setSummary(data ?? null))
        .catch(() => {});
      goApi
        .getLogConnections({ range: filters.range })
        .then((data: LogConnectionSummary[]) =>
          setConnections(Array.isArray(data) ? data : []),
        )
        .catch(() => {});
    };

    es.onerror = () => {
      es.close();
      eventSourceRef.current = null;

      // Exponential backoff reconnect
      const delay = backoffRef.current;
      backoffRef.current = Math.min(backoffRef.current * 2, 30000);

      reconnectTimerRef.current = setTimeout(() => {
        reconnectTimerRef.current = null;
        // Only reconnect if still live
        connectSSE();
      }, delay);
    };

    es.onopen = () => {
      // Reset backoff on successful connection
      backoffRef.current = 1000;
    };

    eventSourceRef.current = es;
  }, [filters, cleanupSSE]);

  useEffect(() => {
    if (live) {
      connectSSE();
    } else {
      cleanupSSE();
    }

    return cleanupSSE;
  }, [live, connectSSE, cleanupSSE]);

  // ── Clear logs ───────────────────────────────────────────────────────────

  const handleClear = useCallback(async () => {
    try {
      await goApi.clearLogs();
      setLogs([]);
      setSummary(null);
      setConnections([]);
      setClearDialogOpen(false);
      toast.success("All logs cleared");
    } catch {
      toast.error("Failed to clear logs");
    }
  }, []);

  // ── Filter handlers ──────────────────────────────────────────────────────

  const handleFiltersChange = useCallback((next: LogFilters) => {
    setFilters(next);
  }, []);

  const handleConnectionSelect = useCallback(
    (connectionId: string) => {
      setFilters({ ...filters, connectionId });
    },
    [filters],
  );

  // ── Render ───────────────────────────────────────────────────────────────

  const safeLogs = Array.isArray(logs) ? logs : [];

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex flex-col gap-1">
        <h1 className="text-2xl font-bold tracking-tight">Logs</h1>
        <p className="text-sm text-muted-foreground">
          Request timeline with payload and cost tracking.
        </p>
      </div>

      {/* Summary cards */}
      <SummaryBar summary={summary} />
      <p className="-mt-2 text-[11px] text-muted-foreground">
        Cost is estimated from local model price profiles; provider billing may
        differ.
      </p>

      {/* Filter bar */}
      <FilterBar
        filters={filters}
        live={live}
        onFiltersChange={handleFiltersChange}
        onLiveChange={setLive}
        onRefresh={handleRefresh}
        onClear={() => setClearDialogOpen(true)}
        refreshing={refreshing}
      />

      {/* Main content: sidebar + log list */}
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start">
        <ConnectionSidebar
          connections={connections}
          selectedId={filters.connectionId}
          onSelect={handleConnectionSelect}
          collapsed={sidebarCollapsed}
          onToggleCollapse={() => setSidebarCollapsed(!sidebarCollapsed)}
        />

        <main className="min-w-0 flex-1 space-y-2">
          <div className="flex items-center justify-between border-b pb-2">
            <div>
              <h3 className="text-sm font-semibold">Request timeline</h3>
              <p className="text-[11px] text-muted-foreground">
                {loading
                  ? "Loading…"
                  : `${safeLogs.length} event${safeLogs.length === 1 ? "" : "s"}`}
                {live && (
                  <span className="ml-1.5 inline-flex items-center gap-1 text-green-500">
                    <span className="relative flex size-1.5">
                      <span className="absolute inline-flex size-full animate-ping rounded-full bg-green-400 opacity-75" />
                      <span className="relative inline-flex size-1.5 rounded-full bg-green-500" />
                    </span>
                    streaming
                  </span>
                )}
              </p>
            </div>
            {safeLogs.length > 0 && (
              <span className="text-[11px] text-muted-foreground">
                {formatDateTime(safeLogs[0]?.timestamp)} –{" "}
                {formatDateTime(safeLogs[safeLogs.length - 1]?.timestamp)}
              </span>
            )}
          </div>

          {loading ? (
            <div className="flex h-64 items-center justify-center">
              <RefreshCw className="size-5 animate-spin text-muted-foreground" />
            </div>
          ) : safeLogs.length === 0 ? (
            <EmptyState />
          ) : (
            <div className="space-y-2">
              {safeLogs.map((log) => (
                <LogEventCard key={log.id} log={log} />
              ))}
            </div>
          )}
        </main>
      </div>

      {/* Clear confirmation dialog */}
      <Dialog open={clearDialogOpen} onOpenChange={setClearDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Clear all logs?</DialogTitle>
            <DialogDescription>
              This will permanently delete all log entries. This action cannot
              be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setClearDialogOpen(false)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleClear}>
              <Trash2 className="size-4" />
              Clear logs
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
