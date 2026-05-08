import { useState, useEffect, useCallback } from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Card, CardContent } from "@/components/ui/card";
import { RefreshCw, ChevronLeft, ChevronRight } from "lucide-react";
import UsageChart from "./usage-chart";
import UsageStats, { type UsageStatsData } from "./usage-stats";

const PERIODS = [
  { value: "24h", label: "24h" },
  { value: "7d", label: "7d" },
  { value: "30d", label: "30d" },
  { value: "60d", label: "60d" },
];

interface RequestDetail {
  timestamp: string;
  model: string;
  provider: string;
  connectionName?: string;
  status: string;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  cost: number;
  durationMs?: number;
}

interface Pagination {
  page: number;
  pageSize: number;
  totalItems: number;
  totalPages: number;
}

function fmtNum(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n || 0);
}

function fmtCost(n: number): string {
  return `$${(n || 0).toFixed(4)}`;
}

function fmtTs(ts: string): string {
  try {
    return new Date(ts).toLocaleString();
  } catch {
    return ts;
  }
}

export default function UsageScreen() {
  const [period, setPeriod] = useState("7d");
  const [tab, setTab] = useState("overview");
  const [stats, setStats] = useState<UsageStatsData | null>(null);
  const [statsLoading, setStatsLoading] = useState(true);
  const [details, setDetails] = useState<RequestDetail[]>([]);
  const [pagination, setPagination] = useState<Pagination>({
    page: 1,
    pageSize: 20,
    totalItems: 0,
    totalPages: 1,
  });
  const [detailsLoading, setDetailsLoading] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);

  const fetchStats = useCallback(async () => {
    setStatsLoading(true);
    try {
      const res = await fetch(`/api/usage/stats?period=${period}`);
      if (res.ok) setStats(await res.json());
    } catch (e) {
      console.error(e);
    } finally {
      setStatsLoading(false);
    }
  }, [period, refreshKey]); // eslint-disable-line react-hooks/exhaustive-deps

  const fetchDetails = useCallback(
    async (page = 1) => {
      setDetailsLoading(true);
      try {
        const res = await fetch(
          `/api/usage/request-details?page=${page}&pageSize=20`
        );
        if (res.ok) {
          const json = await res.json();
          setDetails(json.details || []);
          setPagination(json.pagination);
        }
      } catch (e) {
        console.error(e);
      } finally {
        setDetailsLoading(false);
      }
    },
    [refreshKey] // eslint-disable-line react-hooks/exhaustive-deps
  );

  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  useEffect(() => {
    if (tab === "requests") fetchDetails(1);
  }, [tab, fetchDetails]);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold">Usage Analytics</h1>
          <p className="text-sm text-muted-foreground">
            Token usage and cost breakdown
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1 rounded-lg border bg-muted/30 p-0.5">
            {PERIODS.map((p) => (
              <button
                key={p.value}
                onClick={() => setPeriod(p.value)}
                className={`px-3 py-1 rounded-md text-xs font-medium transition-colors ${
                  period === p.value
                    ? "bg-background shadow-sm text-foreground"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                {p.label}
              </button>
            ))}
          </div>
          <Button
            variant="outline"
            size="icon"
            className="h-8 w-8"
            onClick={() => setRefreshKey((k) => k + 1)}
          >
            <RefreshCw className="w-4 h-4" />
          </Button>
        </div>
      </div>

      <Tabs value={tab} onValueChange={setTab}>
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="requests">Requests</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="flex flex-col gap-4 mt-4">
          <UsageChart period={period} />
          <UsageStats data={stats} loading={statsLoading} />
        </TabsContent>

        <TabsContent value="requests" className="mt-4">
          <Card>
            <CardContent className="p-4">
              <div className="flex items-center justify-between mb-3">
                <h3 className="text-sm font-semibold">Request History</h3>
                <span className="text-xs text-muted-foreground">
                  {pagination.totalItems} total
                </span>
              </div>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Time</TableHead>
                    <TableHead>Model</TableHead>
                    <TableHead>Connection</TableHead>
                    <TableHead className="text-right">Input</TableHead>
                    <TableHead className="text-right">Output</TableHead>
                    <TableHead className="text-right">Cost</TableHead>
                    <TableHead className="text-right">Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {detailsLoading ? (
                    <TableRow>
                      <TableCell colSpan={7} className="text-center text-muted-foreground py-8 text-sm">
                        Loading...
                      </TableCell>
                    </TableRow>
                  ) : details.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="text-center text-muted-foreground py-8 text-sm">
                        No requests found
                      </TableCell>
                    </TableRow>
                  ) : (
                    details.map((d, i) => (
                      <TableRow key={i}>
                        <TableCell className="text-xs whitespace-nowrap">
                          {fmtTs(d.timestamp)}
                        </TableCell>
                        <TableCell className="font-mono text-xs max-w-[160px] truncate">
                          {d.model || "—"}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {d.connectionName || d.provider || "—"}
                        </TableCell>
                        <TableCell className="text-right text-xs">
                          {fmtNum(d.promptTokens)}
                        </TableCell>
                        <TableCell className="text-right text-xs">
                          {fmtNum(d.completionTokens)}
                        </TableCell>
                        <TableCell className="text-right text-xs text-amber-500">
                          {fmtCost(d.cost)}
                        </TableCell>
                        <TableCell className="text-right">
                          <Badge
                            variant={d.status === "error" ? "destructive" : "secondary"}
                            className="text-xs"
                          >
                            {d.status}
                          </Badge>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>

              {pagination.totalPages > 1 && (
                <div className="flex items-center justify-between mt-3 pt-3 border-t">
                  <span className="text-xs text-muted-foreground">
                    Page {pagination.page} of {pagination.totalPages}
                  </span>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="outline"
                      size="icon"
                      className="h-7 w-7"
                      disabled={pagination.page <= 1}
                      onClick={() => fetchDetails(pagination.page - 1)}
                    >
                      <ChevronLeft className="w-3 h-3" />
                    </Button>
                    <Button
                      variant="outline"
                      size="icon"
                      className="h-7 w-7"
                      disabled={pagination.page >= pagination.totalPages}
                      onClick={() => fetchDetails(pagination.page + 1)}
                    >
                      <ChevronRight className="w-3 h-3" />
                    </Button>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
