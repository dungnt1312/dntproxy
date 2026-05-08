import {
  ComposedChart,
  Bar,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDayLabel, type KpiRange } from './helpers'
import type { DailyUsageStat } from '@/types/logs'

// ─── Daily Usage Chart ───────────────────────────────────────────────────────

const CHART_COLORS = {
  requests: 'var(--chart-1)',
  errors: 'var(--destructive)',
  tokens: 'var(--chart-2)',
}

const CHART_RANGE_LABELS: Record<KpiRange, string> = {
  '24h': 'Today',
  '7d': 'Last 7 days',
  '30d': 'Last 30 days',
}

interface DailyUsageChartProps {
  data: DailyUsageStat[]
  loading: boolean
  error: string | null
  range: KpiRange
  onRetry: () => void
}

export function DailyUsageChart({ data, loading, error, range, onRetry }: DailyUsageChartProps) {
  const maxTokens = Math.max(...data.map(d => d.totalTokens), 0)
  const tokenScale = maxTokens >= 1_000_000 ? 1_000_000 : maxTokens >= 1_000 ? 1_000 : 1
  const tokenLabel = maxTokens >= 1_000_000 ? 'Tokens (M)' : maxTokens >= 1_000 ? 'Tokens (K)' : 'Tokens'

  const chartData = data.map(d => ({
    day: formatDayLabel(d.date),
    Requests: d.requests,
    Errors: d.errors,
    [tokenLabel]: tokenScale > 1
      ? Number((d.totalTokens / tokenScale).toFixed(2))
      : d.totalTokens,
  }))

  const hasData = data.some(d => d.requests > 0 || d.totalTokens > 0)

  return (
    <Card className="h-full">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold">Daily Usage</h2>
          <span className="text-[10px] text-muted-foreground uppercase tracking-wider">
            {CHART_RANGE_LABELS[range]}
          </span>
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        {loading ? (
          <Skeleton className="h-[240px] w-full rounded-lg" />
        ) : !hasData ? (
          <div className="flex items-center justify-center h-[240px] text-muted-foreground text-sm">
            No usage data yet
          </div>
        ) : (
          <ResponsiveContainer width="100%" height={240}>
            <ComposedChart data={chartData} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.07)" vertical={false} />
              <XAxis
                dataKey="day"
                tick={{ fontSize: 10, fill: 'rgba(156,163,175,1)' }}
                tickLine={false}
                axisLine={false}
                interval={data.length > 14 ? 4 : data.length > 7 ? 1 : 0}
              />
              <YAxis
                yAxisId="left"
                tick={{ fontSize: 10, fill: 'rgba(156,163,175,1)' }}
                tickLine={false}
                axisLine={false}
                width={32}
              />
              <YAxis
                yAxisId="right"
                orientation="right"
                tick={{ fontSize: 10, fill: 'rgba(156,163,175,1)' }}
                tickLine={false}
                axisLine={false}
                width={42}
                tickFormatter={(v) => tokenScale === 1_000_000 ? `${v}M` : tokenScale === 1_000 ? `${v}K` : String(v)}
              />
              <Tooltip
                contentStyle={{
                  background: 'hsl(var(--popover))',
                  border: '1px solid hsl(var(--border))',
                  borderRadius: '8px',
                  fontSize: 12,
                  color: 'hsl(var(--popover-foreground))',
                }}
                labelStyle={{ fontWeight: 600, marginBottom: 4, color: 'hsl(var(--muted-foreground))' }}
                cursor={{ fill: 'hsl(var(--muted) / 0.3)' }}
              />
              <Legend
                iconType="square"
                iconSize={8}
                wrapperStyle={{ fontSize: 11, paddingTop: 8, color: 'rgba(156,163,175,1)' }}
              />
              <Bar yAxisId="left" dataKey="Requests" fill={CHART_COLORS.requests} radius={[3, 3, 0, 0]} maxBarSize={28} opacity={0.9} />
              <Bar yAxisId="left" dataKey="Errors" fill={CHART_COLORS.errors} radius={[3, 3, 0, 0]} maxBarSize={28} opacity={0.9} />
              <Line
                yAxisId="right"
                type="monotone"
                dataKey={tokenLabel}
                stroke={CHART_COLORS.tokens}
                strokeWidth={2.5}
                dot={false}
                activeDot={{ r: 4, fill: CHART_COLORS.tokens }}
              />
            </ComposedChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  )
}
