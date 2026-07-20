import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDayLabel, formatTokens, type KpiRange } from './helpers'
import type { DailyUsageStat } from '@/types/logs'

// ─── Daily Usage Chart — Stacked Bar by Model ──────────────────────────────────

const MODEL_COLORS = [
  '#6366f1', // indigo
  '#f59e0b', // amber
  '#10b981', // emerald
  '#ef4444', // red
  '#8b5cf6', // violet
  '#06b6d4', // cyan
  '#f97316', // orange
  '#ec4899', // pink
  '#64748b', // slate (Other)
]

const TOP_N = 8

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
  // Compute top models across all days
  const modelTotals: Record<string, number> = {}
  for (const d of data) {
    if (!d.models) continue
    for (const [m, t] of Object.entries(d.models)) {
      modelTotals[m] = (modelTotals[m] || 0) + t
    }
  }
  const topModels = Object.entries(modelTotals)
    .sort((a, b) => b[1] - a[1])
    .slice(0, TOP_N)
    .map(([m]) => m)
  const hasOther = Object.keys(modelTotals).length > TOP_N

  // Pivot: one row per day, each model is a dataKey
  const chartData = data.map(d => {
    const row: Record<string, string | number> = { day: formatDayLabel(d.date) }
    const seen = new Set<string>()
    for (const m of topModels) {
      row[m] = d.models?.[m] ?? 0
      seen.add(m)
    }
    if (hasOther) {
      let other = 0
      if (d.models) {
        for (const [m, t] of Object.entries(d.models)) {
          if (!seen.has(m)) other += t
        }
      }
      row['Other'] = other
    }
    return row
  })

  const modelKeys = [...topModels]
  if (hasOther) modelKeys.push('Other')

  const hasData = data.some(d => d.requests > 0 || d.totalTokens > 0)

  return (
    <Card className="h-full">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold">Tokens by Model</h2>
          <span className="text-[10px] text-muted-foreground uppercase tracking-wider">
            {CHART_RANGE_LABELS[range]}
          </span>
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        {loading ? (
          <Skeleton className="h-[280px] w-full rounded-lg" />
        ) : !hasData ? (
          <div className="flex items-center justify-center h-[280px] text-muted-foreground text-sm">
            No usage data yet
          </div>
        ) : (
          <ResponsiveContainer width="100%" height={280}>
            <BarChart data={chartData} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.07)" vertical={false} />
              <XAxis
                dataKey="day"
                tick={{ fontSize: 10, fill: 'rgba(156,163,175,1)' }}
                tickLine={false}
                axisLine={false}
                interval={data.length > 14 ? 4 : data.length > 7 ? 1 : 0}
              />
              <YAxis
                tick={{ fontSize: 10, fill: 'rgba(156,163,175,1)' }}
                tickLine={false}
                axisLine={false}
                width={42}
                tickFormatter={(v) => formatTokens(v)}
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
                formatter={(value: number, name: string) => [formatTokens(value), name]}
              />
              <Legend
                iconType="square"
                iconSize={8}
                wrapperStyle={{ fontSize: 11, paddingTop: 8, color: 'rgba(156,163,175,1)' }}
              />
              {modelKeys.map((model, i) => (
                <Bar
                  key={model}
                  dataKey={model}
                  stackId="tokens"
                  fill={MODEL_COLORS[i % MODEL_COLORS.length]}
                  radius={i === modelKeys.length - 1 ? [3, 3, 0, 0] : [0, 0, 0, 0]}
                  maxBarSize={40}
                />
              ))}
            </BarChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  )
}
