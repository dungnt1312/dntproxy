import { Badge } from "@/components/ui/badge";

export function StatusBadge({ status, level }: { status?: number; level: string }) {
  if (level === "ERROR" || (status && status >= 400)) {
    return (
      <Badge className="bg-rose-100 text-rose-700 hover:bg-rose-100 dark:bg-rose-950 dark:text-rose-400">
        {status || "ERR"}
      </Badge>
    );
  }
  if (status && status >= 200 && status < 300) {
    return (
      <Badge className="bg-emerald-100 text-emerald-700 hover:bg-emerald-100 dark:bg-emerald-950 dark:text-emerald-400">
        {status}
      </Badge>
    );
  }
  return <Badge variant="secondary">{status || level}</Badge>;
}

export function formatDateTime(dateStr: string) {
  const d = new Date(dateStr);
  if (Number.isNaN(d.getTime())) return dateStr;
  return d.toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

export function formatLatency(ms: number | undefined | null) {
  if (ms == null) return "—";
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}
