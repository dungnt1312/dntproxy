import { RefreshCw } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

export interface QuotaBucket {
  key: string;
  label: string;
  used: number;
  total: number;
  remaining: number;
  pct: number;
  resetAt?: string;
  unlimited: boolean;
}

export interface BillingHistoryEntry {
  year: number;
  month: number;
  includedUsed: number;
  onDemandUsed: number;
  totalUsed: number;
}

export interface UsageData {
  provider?: string;
  plan?: string;
  limitReached?: boolean;
  message?: string;
  quotas?: QuotaBucket[];
  overages?: {
    used: number;
    cap: number;
    remaining: number;
    status?: string;
    charge?: number;
    rate?: number;
  };
  history?: BillingHistoryEntry[];
  error?: string;
}

function timeUntil(dateStr: string): string {
  const diff = new Date(dateStr).getTime() - Date.now();
  if (diff <= 0) return "now";
  const mins = Math.floor(diff / 60000);
  const hours = Math.floor(mins / 60);
  const days = Math.floor(hours / 24);
  if (days > 0) return `in ${days}d ${hours % 24}h`;
  if (hours > 0) return `in ${hours}h ${mins % 60}m`;
  return `in ${mins}m`;
}

function monthLabel(year: number, month: number): string {
  if (!year || !month) return "—";
  // month from API is 1-12
  const d = new Date(Date.UTC(year, month - 1, 1));
  return d.toLocaleString(undefined, { month: "short", year: "numeric", timeZone: "UTC" });
}

interface QuotaPanelProps {
  data: UsageData | null;
  loading?: boolean;
  onRefresh?: (e?: React.MouseEvent) => void;
}

export default function QuotaPanel({
  data,
  loading,
  onRefresh,
}: QuotaPanelProps) {
  if (loading && !data) {
    return (
      <div className="flex flex-col gap-1.5 justify-center py-1">
        <div className="flex items-center gap-2 text-muted-foreground">
          <RefreshCw className="h-3 w-3 animate-spin" />
          <span className="text-[11px]">Fetching quota...</span>
        </div>
      </div>
    );
  }

  if (!data) return null;

  if (data.error) {
    return (
      <div className="flex items-center justify-between gap-2">
        <p className="text-[11px] text-destructive flex-1">{data.error}</p>
        {onRefresh && (
          <button
            type="button"
            className="p-1 rounded-md hover:bg-muted/80 text-muted-foreground hover:text-foreground cursor-pointer transition-colors shrink-0"
            onClick={onRefresh}
            title="Retry"
            aria-label="Retry quota fetch"
          >
            <RefreshCw size={12} className={cn(loading && "animate-spin")} />
          </button>
        )}
      </div>
    );
  }

  const quotas = data.quotas ?? [];
  if (data.message && quotas.length === 0 && !data.overages && !(data.history?.length)) {
    return (
      <div className="flex items-center justify-between gap-2">
        <p className="text-[11px] text-muted-foreground italic flex-1 leading-tight">
          {data.message}
        </p>
        {onRefresh && (
          <button
            type="button"
            className="p-1 rounded-md hover:bg-muted/80 text-muted-foreground hover:text-foreground cursor-pointer transition-colors shrink-0"
            onClick={onRefresh}
            title="Refresh"
            aria-label="Refresh quota"
          >
            <RefreshCw size={12} className={cn(loading && "animate-spin")} />
          </button>
        )}
      </div>
    );
  }

  if (quotas.length === 0 && !data.overages && !(data.history?.length)) {
    return (
      <div className="flex items-center justify-between gap-2">
        <p className="text-[11px] text-muted-foreground italic flex-1">
          No quota info available.
        </p>
        {onRefresh && (
          <button
            type="button"
            className="p-1 rounded-md hover:bg-muted/80 text-muted-foreground hover:text-foreground cursor-pointer transition-colors shrink-0"
            onClick={onRefresh}
            title="Refresh"
            aria-label="Refresh quota"
          >
            <RefreshCw size={12} className={cn(loading && "animate-spin")} />
          </button>
        )}
      </div>
    );
  }

  // pct is percent USED (0-100)
  const usedRiskColor = (usedPct: number) =>
    usedPct >= 90 ? "bg-red-500" : usedPct >= 70 ? "bg-amber-500" : "bg-emerald-500";

  const history = data.history ?? [];

  return (
    <div className="space-y-1.5 w-full">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 min-w-0">
          {data.plan && (
            <span className="text-[10px] text-muted-foreground font-medium uppercase tracking-widest truncate">
              {data.plan}
            </span>
          )}
          {data.limitReached && (
            <Badge
              variant="outline"
              className="text-destructive border-destructive/30 bg-destructive/10 text-[9px] px-1 py-0 h-4 shrink-0"
            >
              Limit Reached
            </Badge>
          )}
        </div>
        {onRefresh && (
          <button
            type="button"
            className="p-1 -mr-1 rounded hover:bg-muted/60 text-muted-foreground hover:text-foreground cursor-pointer transition-colors"
            onClick={onRefresh}
            title="Refresh quota"
            aria-label="Refresh quota"
          >
            <RefreshCw size={12} className={cn(loading && "animate-spin")} />
          </button>
        )}
      </div>

      <div className="space-y-2">
        {quotas.map((b) => (
          <div key={b.key} className="space-y-1.5">
            <div className="flex items-end justify-between text-xs leading-none gap-2">
              <span className="text-muted-foreground/80 font-medium capitalize text-[11px]">
                {b.label}
              </span>
              <div className="flex items-center gap-1.5 shrink-0">
                {b.unlimited ? (
                  <span className="font-mono text-[11px] text-foreground/80 font-medium">
                    Unlimited
                  </span>
                ) : (
                  <span
                    className="font-mono text-[11px] text-foreground/80 font-medium"
                    title={`${b.used} / ${b.total} (${b.pct}%) · remaining ${b.remaining}`}
                  >
                    {b.used}
                    <span className="text-muted-foreground/50 text-[10px]"> / {b.total}</span>
                  </span>
                )}
                {b.resetAt && (
                  <span className="text-[10px] text-muted-foreground/50">
                    · {timeUntil(b.resetAt)}
                  </span>
                )}
              </div>
            </div>
            {!b.unlimited && (
              <div className="relative h-1.5 rounded-full bg-muted/60 overflow-hidden w-full">
                <div
                  className={`absolute inset-y-0 left-0 rounded-full transition-all duration-500 ${usedRiskColor(b.pct)}`}
                  style={{ width: `${Math.min(100, Math.max(0, b.pct))}%` }}
                />
              </div>
            )}
          </div>
        ))}

        {data.overages && (
          <div className="space-y-1.5 pt-1 border-t border-muted-foreground/10">
            <div className="flex items-end justify-between text-xs leading-none gap-2">
              <div className="flex items-center gap-1.5">
                <span className="text-muted-foreground/80 font-medium text-[11px]">
                  On-demand
                </span>
                <Badge
                  variant="outline"
                  className={cn(
                    "text-[9px] px-1 py-0 h-4",
                    data.overages.cap > 0
                      ? "border-amber-500/30 bg-amber-500/10 text-amber-500"
                      : "border-muted-foreground/20 bg-muted/40 text-muted-foreground",
                  )}
                >
                  {data.overages.status || (data.overages.cap > 0 ? "ENABLED" : "DISABLED")}
                </Badge>
              </div>
              <div className="flex items-center gap-1.5">
                {data.overages.cap > 0 ? (
                  <span className="font-mono text-[11px] text-amber-500 font-medium">
                    {data.overages.used.toFixed(0)}
                    <span className="text-muted-foreground/50 text-[10px]">
                      {" "}
                      / {data.overages.cap.toFixed(0)}
                    </span>
                  </span>
                ) : (
                  <span className="text-[10px] text-muted-foreground/60">cap 0</span>
                )}
                {data.overages.charge != null && data.overages.charge > 0 && (
                  <span className="text-[10px] text-amber-500/70">
                    ${data.overages.charge.toFixed(4)}
                  </span>
                )}
              </div>
            </div>
            {data.overages.cap > 0 && (
              <div className="relative h-1.5 rounded-full bg-muted/60 overflow-hidden w-full">
                <div
                  className="absolute inset-y-0 left-0 rounded-full transition-all duration-500 bg-amber-500"
                  style={{
                    width: `${Math.min(100, (data.overages.used / data.overages.cap) * 100)}%`,
                  }}
                />
              </div>
            )}
            {data.overages.rate != null && data.overages.rate > 0 && (
              <div className="text-[10px] text-muted-foreground/50">
                Rate: ${data.overages.rate.toFixed(4)}/unit · Remaining:{" "}
                {data.overages.remaining.toFixed(2)}
              </div>
            )}
          </div>
        )}

        {history.length > 0 && (
          <div className="pt-1 border-t border-muted-foreground/10 space-y-1">
            <span className="text-[10px] text-muted-foreground/70 font-medium uppercase tracking-wider">
              History
            </span>
            <div className="space-y-0.5">
              {history.slice(0, 6).map((h) => (
                <div
                  key={`${h.year}-${h.month}`}
                  className="flex items-center justify-between text-[10px] text-muted-foreground/80 font-mono"
                >
                  <span>{monthLabel(h.year, h.month)}</span>
                  <span title={`included ${h.includedUsed} · on-demand ${h.onDemandUsed}`}>
                    {h.totalUsed}
                    <span className="text-muted-foreground/40"> total</span>
                    {h.onDemandUsed > 0 && (
                      <span className="text-amber-500/80"> · od {h.onDemandUsed}</span>
                    )}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
