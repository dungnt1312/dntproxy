import { useCallback, useEffect, useRef, useState } from "react";
import { Pause, Play, Trash2, Search, ArrowDown } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { getStoredApiKey } from "@/lib/go-api";
import type { LogEntry } from "@/types/logs";
import { buildFilterParams } from "./helpers";
import type { LogFilters } from "@/types/logs";

const SSE_BASE = import.meta.env.VITE_GO_API_URL || "/api";
const MAX_LINES = 2000;

// Color mapping for log levels
function getLevelColor(level: string): string {
  switch (level?.toUpperCase()) {
    case "ERROR":
      return "text-red-400";
    case "WARN":
      return "text-yellow-400";
    case "INFO":
      return "text-green-400";
    default:
      return "text-gray-400";
  }
}

// Color mapping for providers
function getProviderColor(provider: string): string {
  switch (provider?.toUpperCase()) {
    case "KIRO":
      return "text-cyan-400";
    case "OPENAI":
      return "text-emerald-400";
    case "ANTHROPIC":
      return "text-orange-400";
    case "GLM":
      return "text-blue-400";
    case "MINIMAX":
      return "text-purple-400";
    case "QWEN":
      return "text-pink-400";
    default:
      return "text-gray-500";
  }
}

function formatTime(timestamp: string): string {
  try {
    const d = new Date(timestamp);
    return d.toLocaleTimeString("en-US", { hour12: false, hour: "2-digit", minute: "2-digit", second: "2-digit" });
  } catch {
    return "??:??:??";
  }
}

function formatLogLine(log: LogEntry): { time: string; level: string; provider: string; message: string } {
  return {
    time: formatTime(log.timestamp),
    level: log.level?.toUpperCase() || "INFO",
    provider: log.provider || "APP",
    message: log.message || "",
  };
}

interface ConsoleViewerProps {
  filters: LogFilters;
}

export function ConsoleViewer({ filters }: ConsoleViewerProps) {
  const [lines, setLines] = useState<LogEntry[]>([]);
  const [paused, setPaused] = useState(false);
  const [searchText, setSearchText] = useState("");
  const [showSearch, setShowSearch] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);

  const containerRef = useRef<HTMLDivElement>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const backoffRef = useRef(1000);
  const pausedLinesRef = useRef<LogEntry[]>([]);

  // Auto-scroll to bottom
  useEffect(() => {
    if (autoScroll && !paused && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [lines, autoScroll, paused]);

  // Detect user scroll to pause auto-scroll
  const handleScroll = useCallback(() => {
    if (!containerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 40;
    setAutoScroll(isAtBottom);
  }, []);

  // SSE connection
  useEffect(() => {
    const connectSSE = () => {
      if (eventSourceRef.current) eventSourceRef.current.close();

      const params = buildFilterParams(filters);
      const apiKey = getStoredApiKey();
      if (apiKey) params.set("key", apiKey);
      const url = `${SSE_BASE}/logs/stream?${params.toString()}`;

      const sse = new EventSource(url);
      eventSourceRef.current = sse;

      sse.onopen = () => {
        backoffRef.current = 1000;
      };

      sse.onmessage = (event) => {
        try {
          const rawData = event.data;
          if (rawData === ": keepalive") return;

          const data = JSON.parse(rawData);
          if (data.type === "init" && Array.isArray(data.logs)) {
            const sorted = [...data.logs].reverse(); // oldest first for console
            setLines(sorted.slice(-MAX_LINES));
          } else if (data.type === "delta" && data.log) {
            if (paused) {
              pausedLinesRef.current.push(data.log);
            } else {
              setLines((prev) => {
                const next = [...prev, data.log];
                return next.length > MAX_LINES ? next.slice(-MAX_LINES) : next;
              });
            }
          }
        } catch (e) {
          console.error("Console SSE parse error", e);
        }
      };

      sse.onerror = () => {
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
  }, [filters, paused]);

  // Resume: flush buffered lines
  const handleTogglePause = () => {
    if (paused) {
      setLines((prev) => {
        const merged = [...prev, ...pausedLinesRef.current];
        pausedLinesRef.current = [];
        return merged.length > MAX_LINES ? merged.slice(-MAX_LINES) : merged;
      });
      setAutoScroll(true);
    }
    setPaused(!paused);
  };

  const handleClear = () => {
    setLines([]);
    pausedLinesRef.current = [];
  };

  const scrollToBottom = () => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
      setAutoScroll(true);
    }
  };

  // Filter lines by search text
  const filteredLines = searchText
    ? lines.filter((l) => l.message?.toLowerCase().includes(searchText.toLowerCase()) ||
        l.provider?.toLowerCase().includes(searchText.toLowerCase()) ||
        l.level?.toLowerCase().includes(searchText.toLowerCase()))
    : lines;

  return (
    <div className="flex flex-col h-full min-h-0 rounded-lg border border-border overflow-hidden">
      {/* Toolbar */}
      <div className="flex items-center gap-2 px-3 py-2 bg-muted/50 border-b border-border">
        <Button
          variant="ghost"
          size="sm"
          onClick={handleTogglePause}
          className="h-7 px-2 text-muted-foreground hover:text-foreground"
        >
          {paused ? <Play className="h-3.5 w-3.5" /> : <Pause className="h-3.5 w-3.5" />}
          <span className="ml-1.5 text-xs">{paused ? "Resume" : "Pause"}</span>
        </Button>

        <Button
          variant="ghost"
          size="sm"
          onClick={handleClear}
          className="h-7 px-2 text-muted-foreground hover:text-foreground"
        >
          <Trash2 className="h-3.5 w-3.5" />
          <span className="ml-1.5 text-xs">Clear</span>
        </Button>

        <Button
          variant="ghost"
          size="sm"
          onClick={() => setShowSearch(!showSearch)}
          className={cn("h-7 px-2 text-muted-foreground hover:text-foreground", showSearch && "bg-accent")}
        >
          <Search className="h-3.5 w-3.5" />
          <span className="ml-1.5 text-xs">Filter</span>
        </Button>

        {showSearch && (
          <Input
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            placeholder="Filter logs..."
            className="h-7 w-48 text-xs"
          />
        )}

        <div className="flex-1" />

        {paused && (
          <span className="text-xs text-yellow-500 dark:text-yellow-400 font-medium">
            PAUSED ({pausedLinesRef.current.length} buffered)
          </span>
        )}

        <span className="text-xs text-muted-foreground">{filteredLines.length} lines</span>
      </div>

      {/* Console output — always dark for terminal feel */}
      <div
        ref={containerRef}
        onScroll={handleScroll}
        className="flex-1 min-h-0 overflow-y-auto bg-[#0a0a0f] p-3 font-mono text-xs leading-5 select-text"
      >
        {filteredLines.length === 0 ? (
          <div className="flex items-center justify-center h-full text-zinc-600">
            {searchText ? "No matching logs" : "Waiting for logs..."}
          </div>
        ) : (
          filteredLines.map((log) => {
            const { time, level, provider, message } = formatLogLine(log);
            return (
              <div key={log.id} className="whitespace-pre-wrap break-all hover:bg-white/5">
                <span className="text-zinc-500">[{time}]</span>{" "}
                <span className={cn("font-semibold", getLevelColor(level))}>[{level}]</span>{" "}
                <span className={cn("font-medium", getProviderColor(provider))}>[{provider}]</span>{" "}
                <span className="text-zinc-200">{message}</span>
              </div>
            );
          })
        )}
      </div>

      {/* Scroll-to-bottom indicator */}
      {!autoScroll && !paused && (
        <button
          onClick={scrollToBottom}
          className="absolute bottom-4 right-4 flex items-center gap-1 px-2 py-1 rounded bg-primary text-primary-foreground text-xs hover:bg-primary/90 shadow-lg"
        >
          <ArrowDown className="h-3 w-3" />
          New logs
        </button>
      )}
    </div>
  );
}
