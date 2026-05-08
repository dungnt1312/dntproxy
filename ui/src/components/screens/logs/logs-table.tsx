import { ChevronLeft, ChevronRight, Clock, Zap, FileWarning } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Separator } from "@/components/ui/separator";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { StatusBadge, formatDateTime, formatLatency } from "./helpers";
import type { LogEntry } from "@/types/logs";

export interface LogsTableProps {
  logs: LogEntry[];
  isLoading: boolean;
  page: number;
  limit: number;
  onPageChange: (page: number) => void;
  onLogSelect: (logId: string) => void;
  hasActiveFilters: boolean;
}

export function LogsTable({
  logs,
  isLoading,
  page,
  limit,
  onPageChange,
  onLogSelect,
  hasActiveFilters,
}: LogsTableProps) {
  const total = logs.length;
  const totalPages = Math.max(1, Math.ceil(total / limit));
  const startIdx = (page - 1) * limit;
  const endIdx = startIdx + limit;
  const displayedLogs = logs.slice(startIdx, endIdx);

  if (isLoading) {
    return (
      <div className="flex-1 rounded-lg border bg-card flex flex-col overflow-hidden min-h-[400px]">
        <div className="p-4 space-y-3">
          {[...Array(6)].map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      </div>
    );
  }

  if (displayedLogs.length === 0) {
    return (
      <div className="flex-1 rounded-lg border bg-card flex flex-col overflow-hidden min-h-[400px]">
        <div className="flex-1 flex flex-col items-center justify-center py-16 text-center">
          <Clock className="h-10 w-10 text-muted-foreground/50 mb-3" />
          <p className="text-muted-foreground font-medium">No logs found</p>
          <p className="text-sm text-muted-foreground/70 mt-1">
            {hasActiveFilters
              ? "Try adjusting your filters"
              : "Request logs will appear here when requests are made"}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 rounded-lg border bg-card flex flex-col overflow-hidden min-h-[400px]">
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
                onClick={() => onLogSelect(log.id)}
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
                onClick={() => onPageChange(page - 1)}
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
                onClick={() => onPageChange(page + 1)}
              >
                <ChevronRight className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
