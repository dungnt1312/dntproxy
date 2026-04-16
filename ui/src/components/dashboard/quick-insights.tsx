import { useMemo } from 'react'
import { Activity, Clock, Coins, TrendingDown, TrendingUp, Minus } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  BarChart,
  Bar,
  ResponsiveContainer,
  Cell,
} from 'recharts'

interface QuickInsightsProps {
  totalRequests: number
  avgLatencyMs: number
  costTotal: number
  requestsByHour: { hour: string; count: number }[]
  loading: boolean
}

function formatCost(n: number): string {
  if (n >= 1) return `$${n.toFixed(2)}`
  if (n >= 0.01) return `$${n.toFixed(3)}`
  if (n > 0) return `$${n.toFixed(4)}`
  return '$0'
}

function formatLatency(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

function SparklineChart({ data, color = '#10b981' }: { data: number[]; color?: string }) {
  const chartData = data.map((v, i) => ({ i, v }))
  const maxVal = Math.max(...data, 1)

  return (
    <ResponsiveContainer width="100%" height={32}>
      <BarChart data={chartData} margin={{ top: 2, right: 0, left: 0, bottom: 2 }}>
        <Bar dataKey="v" radius={[2, 2, 0, 0]}>
          {chartData.map((entry, idx) => (
            <Cell
              key={idx}
              fill={entry.v >= maxVal * 0.8 ? color : color}
              fillOpacity={entry.v >= maxVal * 0.8 ? 0.9 : 0.4}
            />
          ))}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  )
}

function TrendIndicator({ value, invert = false }: { value: number; invert?: boolean }) {
  if (Math.abs(value) < 1) {
    return <Minus className="size-3 text-muted-foreground" />
  }
  const isPositive = invert ? value < 0 : value > 0
  return isPositive
    ? <TrendingUp className="size-3 text-emerald-500" />
    : <TrendingDown className="size-3 text-red-500" />
}

export function QuickInsights({ totalRequests, avgLatencyMs, costTotal, requestsByHour, loading }: QuickInsightsProps) {
  const sparklineData = useMemo(() => {
    if (!requestsByHour?.length) return Array(24).fill(0)
    return requestsByHour.map(h => h.count)
  }, [requestsByHour])

  const latencyStatus = useMemo(() => {
    if (avgLatencyMs < 500) return { color: 'text-emerald-600 dark:text-emerald-400', label: 'Fast' }
    if (avgLatencyMs < 1500) return { color: 'text-amber-600 dark:text-amber-400', label: 'Moderate' }
    return { color: 'text-red-600 dark:text-red-400', label: 'Slow' }
  }, [avgLatencyMs])

  if (loading) {
    return (
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        {[1, 2, 3].map(i => (
          <Card key={i}>
            <CardContent className="p-4">
              <Skeleton className="h-4 w-20 mb-3" />
              <Skeleton className="h-7 w-24 mb-2" />
              <Skeleton className="h-8 w-full" />
            </CardContent>
          </Card>
        ))}
      </div>
    )
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
      <Card className="group cursor-pointer hover:shadow-md transition-shadow">
        <CardContent className="p-4">
          <div className="flex items-center justify-between mb-1">
            <span className="text-xs text-muted-foreground font-medium">Requests</span>
            <Activity className="size-3.5 text-muted-foreground" />
          </div>
          <div className="text-xl font-bold tabular-nums">{totalRequests.toLocaleString()}</div>
          <div className="mt-2">
            <SparklineChart data={sparklineData} color="#3b82f6" />
          </div>
        </CardContent>
      </Card>

      <Card className="group cursor-pointer hover:shadow-md transition-shadow">
        <CardContent className="p-4">
          <div className="flex items-center justify-between mb-1">
            <span className="text-xs text-muted-foreground font-medium">Latency</span>
            <Clock className="size-3.5 text-muted-foreground" />
          </div>
          <div className="flex items-baseline gap-2">
            <span className="text-xl font-bold tabular-nums">{formatLatency(avgLatencyMs)}</span>
            <span className={`text-xs font-medium ${latencyStatus.color}`}>{latencyStatus.label}</span>
          </div>
          <div className="mt-3 h-2 rounded-full bg-muted overflow-hidden">
            <div
              className={cn(
                'h-full rounded-full transition-all duration-500',
                avgLatencyMs < 500 && 'bg-emerald-500',
                avgLatencyMs >= 500 && avgLatencyMs < 1500 && 'bg-amber-500',
                avgLatencyMs >= 1500 && 'bg-red-500',
              )}
              style={{ width: `${Math.min((avgLatencyMs / 3000) * 100, 100)}%` }}
            />
          </div>
        </CardContent>
      </Card>

      <Card className="group cursor-pointer hover:shadow-md transition-shadow">
        <CardContent className="p-4">
          <div className="flex items-center justify-between mb-1">
            <span className="text-xs text-muted-foreground font-medium">Cost (24h)</span>
            <Coins className="size-3.5 text-muted-foreground" />
          </div>
          <div className="flex items-baseline gap-2">
            <span className="text-xl font-bold tabular-nums">{formatCost(costTotal)}</span>
            {costTotal > 0 && (
              <span className="text-xs text-muted-foreground">estimated</span>
            )}
          </div>
          <div className="mt-3 flex items-center gap-1.5 text-xs text-muted-foreground">
            <span>Based on token usage & model pricing</span>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function cn(...classes: (string | boolean | undefined)[]) {
  return classes.filter(Boolean).join(' ')
}
