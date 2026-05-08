import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

export interface UsageGroup {
  key: string;
  label?: string;
  requests: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  inputCost: number;
  outputCost: number;
  totalCost: number;
}

export interface UsageStatsData {
  period: string;
  totalRequests: number;
  totalPromptTokens: number;
  totalCompletionTokens: number;
  totalCost: number;
  byProvider: UsageGroup[] | null;
  byModel: UsageGroup[] | null;
  byConnection: UsageGroup[] | null;
}

function fmtNum(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n || 0);
}

function fmtCost(n: number): string {
  return `$${(n || 0).toFixed(4)}`;
}

function SummaryCard({
  label,
  value,
  sub,
}: {
  label: string;
  value: string;
  sub?: string;
}) {
  return (
    <Card>
      <CardContent className="p-4">
        <p className="text-xs text-muted-foreground mb-1">{label}</p>
        <p className="text-2xl font-bold">{value}</p>
        {sub && <p className="text-xs text-muted-foreground mt-0.5">{sub}</p>}
      </CardContent>
    </Card>
  );
}

function GroupTable({ title, rows }: { title: string; rows: UsageGroup[] }) {
  if (!rows || rows.length === 0) return null;
  return (
    <Card>
      <CardContent className="p-4">
        <h3 className="text-sm font-semibold mb-3">{title}</h3>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead className="text-right">Requests</TableHead>
              <TableHead className="text-right">Input</TableHead>
              <TableHead className="text-right">Output</TableHead>
              <TableHead className="text-right">Total Tokens</TableHead>
              <TableHead className="text-right">Cost</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={row.key}>
                <TableCell className="font-mono text-xs">
                  {row.label || row.key || "—"}
                </TableCell>
                <TableCell className="text-right text-xs">
                  {row.requests}
                </TableCell>
                <TableCell className="text-right text-xs">
                  {fmtNum(row.promptTokens)}
                </TableCell>
                <TableCell className="text-right text-xs">
                  {fmtNum(row.completionTokens)}
                </TableCell>
                <TableCell className="text-right text-xs font-medium">
                  {fmtNum(row.totalTokens)}
                </TableCell>
                <TableCell className="text-right text-xs text-amber-500">
                  {fmtCost(row.totalCost)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}

interface UsageStatsProps {
  data: UsageStatsData | null;
  loading: boolean;
}

export default function UsageStats({ data, loading }: UsageStatsProps) {
  if (loading) {
    return (
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[...Array(4)].map((_, i) => (
          <Card key={i}>
            <CardContent className="p-4">
              <div className="h-4 bg-muted rounded animate-pulse mb-2 w-20" />
              <div className="h-7 bg-muted rounded animate-pulse w-28" />
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  if (!data) return null;

  const totalTokens = (data.totalPromptTokens || 0) + (data.totalCompletionTokens || 0);

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <SummaryCard label="Total Requests" value={String(data.totalRequests || 0)} />
        <SummaryCard
          label="Prompt Tokens"
          value={fmtNum(data.totalPromptTokens || 0)}
        />
        <SummaryCard
          label="Completion Tokens"
          value={fmtNum(data.totalCompletionTokens || 0)}
          sub={`Total: ${fmtNum(totalTokens)}`}
        />
        <SummaryCard
          label="Estimated Cost"
          value={fmtCost(data.totalCost || 0)}
          sub="USD"
        />
      </div>

      <GroupTable title="By Provider" rows={data.byProvider ?? []} />
      <GroupTable title="By Model" rows={data.byModel ?? []} />
      <GroupTable title="By Connection" rows={data.byConnection ?? []} />
    </div>
  );
}
