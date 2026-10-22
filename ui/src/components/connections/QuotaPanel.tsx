import { QuotaBar, StatusBadge } from "@/components/screens/connections/status-ui";
import type { QuotaBucket, UsageData } from "@/types/connections";

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
  const d = new Date(Date.UTC(year, month - 1, 1));
  return d.toLocaleString(undefined, { month: "short", year: "numeric", timeZone: "UTC" });
}

function formatQuotaValue(bucket: QuotaBucket, value: number): string {
  if (bucket.unit === "credits" && bucket.scale) return `$${(value / bucket.scale).toFixed(3)}`;
  return value.toLocaleString();
}

/** Read-only quota breakdown. Loading/refresh/error chrome is owned by the caller. */
export default function QuotaPanel({ data }: { data: UsageData }) {
  const quotas = data.quotas ?? [];
  const history = data.history ?? [];

  if (quotas.length === 0 && !data.overages && history.length === 0) {
    return <p className="text-sm text-muted-foreground">{data.message || "No quota info available."}</p>;
  }

  return (
    <div className="w-full space-y-3">
      {(data.plan || data.limitReached) && (
        <div className="flex items-center gap-2">
          {data.plan && <span className="truncate text-xs font-medium uppercase tracking-wide text-muted-foreground">{data.plan}</span>}
          {data.limitReached && <StatusBadge tone="danger" className="h-5 px-1.5">Limit reached</StatusBadge>}
        </div>
      )}

      {quotas.map((b) => (
        <div key={b.key} className="space-y-1.5">
          <div className="flex items-baseline justify-between gap-2 text-sm">
            <span className="capitalize text-muted-foreground">{b.label}</span>
            <span className="shrink-0 font-mono text-xs tabular-nums">
              {b.unlimited ? (
                "Unlimited"
              ) : (
                <span title={`Remaining ${formatQuotaValue(b, b.remaining)}`}>
                  {formatQuotaValue(b, b.used)}
                  <span className="text-muted-foreground"> / {formatQuotaValue(b, b.total)}</span>
                </span>
              )}
              {b.resetAt && <span className="text-muted-foreground"> · resets {timeUntil(b.resetAt)}</span>}
            </span>
          </div>
          {!b.unlimited && <QuotaBar pct={b.pct} className="w-full" />}
        </div>
      ))}

      {data.overages && (
        <div className="space-y-1.5 border-t pt-3">
          <div className="flex items-baseline justify-between gap-2 text-sm">
            <span className="flex items-center gap-2 text-muted-foreground">
              On-demand
              <StatusBadge tone={data.overages.cap > 0 ? "warning" : "neutral"} className="h-5 px-1.5">
                {data.overages.status || (data.overages.cap > 0 ? "Enabled" : "Disabled")}
              </StatusBadge>
            </span>
            <span className="font-mono text-xs tabular-nums">
              {data.overages.cap > 0 ? (
                <>
                  {data.overages.used.toFixed(0)}
                  <span className="text-muted-foreground"> / {data.overages.cap.toFixed(0)}</span>
                </>
              ) : (
                <span className="text-muted-foreground">cap 0</span>
              )}
              {data.overages.charge != null && data.overages.charge > 0 && (
                <span className="text-warning"> · ${data.overages.charge.toFixed(4)}</span>
              )}
            </span>
          </div>
          {data.overages.cap > 0 && (
            <QuotaBar pct={(data.overages.used / data.overages.cap) * 100} className="w-full" />
          )}
          {data.overages.rate != null && data.overages.rate > 0 && (
            <p className="text-xs text-muted-foreground">
              Rate ${data.overages.rate.toFixed(4)}/unit · Remaining {data.overages.remaining.toFixed(2)}
            </p>
          )}
        </div>
      )}

      {history.length > 0 && (
        <div className="space-y-1 border-t pt-3">
          <span className="text-xs font-medium text-muted-foreground">History</span>
          <ul className="space-y-0.5">
            {history.slice(0, 6).map((h) => (
              <li key={`${h.year}-${h.month}`} className="flex justify-between font-mono text-xs tabular-nums text-muted-foreground">
                <span>{monthLabel(h.year, h.month)}</span>
                <span title={`included ${h.includedUsed} · on-demand ${h.onDemandUsed}`}>
                  <span className="text-foreground">{h.totalUsed}</span> total
                  {h.onDemandUsed > 0 && <span className="text-warning"> · od {h.onDemandUsed}</span>}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
