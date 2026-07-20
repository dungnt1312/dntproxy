import { Filter, X, Radio, RefreshCw } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { LogFilters, LogConnectionSummary } from "@/types/logs";

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
  { value: "CLINE", label: "ClinePass" },
  { value: "GEMINI", label: "Gemini" },
  { value: "OAI_COMPAT", label: "OAI Compatible" },
  { value: "GLM", label: "Zhipu GLM" },
  { value: "QWEN", label: "Alibaba Qwen" },
  { value: "MINIMAX", label: "MiniMax" },
];

export interface FilterBarProps {
  filters: LogFilters;
  onFiltersChange: (filters: LogFilters) => void;
  connections: LogConnectionSummary[];
  hiddenFilters?: (keyof LogFilters)[];
  embedded?: boolean;
  allowedProviders?: string[];
  live: boolean;
  onLiveChange: (live: boolean) => void;
  onRefresh: () => void;
  isRefreshing: boolean;
  hasActiveFilters: boolean;
  onClearFilters: () => void;
}

export function FilterBar({
  filters,
  onFiltersChange,
  connections,
  hiddenFilters = [],
  embedded = false,
  allowedProviders,
  live,
  onLiveChange,
  onRefresh,
  isRefreshing,
  hasActiveFilters,
  onClearFilters,
}: FilterBarProps) {
  const filteredProviderOptions = PROVIDER_OPTIONS.filter(
    (o) => !allowedProviders || o.value === "all" || o.value === "CLIENT" || allowedProviders.includes(o.value)
  );

  return (
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
          onValueChange={(v) => onFiltersChange({ ...filters, range: v })}
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
          onValueChange={(v) => onFiltersChange({ ...filters, provider: v })}
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
          onValueChange={(v) => onFiltersChange({ ...filters, connectionId: v })}
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
          onChange={(e) => onFiltersChange({ ...filters, q: e.target.value })}
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
                onFiltersChange({ ...filters, level: checked ? "ERROR" : "all" });
              }}
            />
            <span className="text-xs text-muted-foreground hidden lg:inline">
              Errors only
            </span>
          </label>
        )}

        {hasActiveFilters && (
          <Button
            variant="ghost"
            size="sm"
            onClick={onClearFilters}
            className="h-8 px-2 text-muted-foreground hover:bg-muted/50"
          >
            <X className="mr-1 h-3 w-3" />
            <span className="hidden sm:inline">Clear</span>
          </Button>
        )}

        {embedded && (
          <>
            <Separator orientation="vertical" className="h-4 mx-1" />
            <div className="flex items-center gap-1.5 px-2 bg-muted/30 rounded-md border py-1">
              <Radio
                className={cn(
                  "h-3.5 w-3.5",
                  live ? "text-green-500 animate-pulse" : "text-muted-foreground"
                )}
              />
              <span className="text-xs font-medium hidden sm:inline mr-1">Live</span>
              <Switch checked={live} onCheckedChange={onLiveChange} className="scale-75 origin-right" />
            </div>
            <Button
              variant="outline"
              size="icon"
              onClick={onRefresh}
              disabled={isRefreshing || live}
              className="h-7 w-7"
            >
              <RefreshCw className={cn("h-3.5 w-3.5", isRefreshing && "animate-spin")} />
            </Button>
          </>
        )}
      </div>
    </div>
  );
}
