import { useMemo } from 'react'
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip as RechartsTooltip,
  Cell,
} from 'recharts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Zap, Coins, CheckCircle2 } from 'lucide-react'
import type { LogConnectionSummary } from '@/types/logs'

const PROVIDER_COLORS: Record<string, string> = {
  kiro: '#ef4444',
  openai: '#10b981',
  'openai-compatible': '#6b7280',
  anthropic: '#f59e0b',
  glm: '#8b5cf6',
  minimax: '#ec4899',
  qwen: '#3b82f6',
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

const tooltipStyle = {
  backgroundColor: 'hsl(var(--popover))',
  borderColor: 'hsl(var(--border))',
  borderRadius: '8px',
  fontSize: '12px',
}

interface ConnectionUsageChartsProps {
  summaries: LogConnectionSummary[]
  loading?: boolean
}

export function ConnectionUsageCharts({ summaries, loading }: ConnectionUsageChartsProps) {
  const sorted = useMemo(
    () => [...summaries].sort((a, b) => b.requests - a.requests),
    [summaries],
  )

  const tokenData = useMemo(
    () =>
      sorted.map(s => ({
        name: s.connectionName || s.connectionId.slice(0, 8),
        input: s.inputTokens,
        output: s.outputTokens,
        total: s.totalTokens,
      })),
    [sorted],
  )

  const requestData = useMemo(
    () =>
      sorted.map(s => ({
        name: s.connectionName || s.connectionId.slice(0, 8),
        requests: s.requests,
        provider: s.provider,
      })),
    [sorted],
  )

  const successData = useMemo(
    () =>
      sorted
        .filter(s => s.requests > 0)
        .map(s => ({
          name: s.connectionName || s.connectionId.slice(0, 8),
          rate: +(((s.requests - s.errors) / s.requests) * 100).toFixed(1),
          errors: s.errors,
          total: s.requests,
        }))
        .sort((a, b) => b.rate - a.rate),
    [sorted],
  )

  if (loading) {
    return (
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        {[1, 2, 3].map(i => (
          <Card key={i}>
            <CardHeader className="pb-2">
              <Skeleton className="h-5 w-32" />
            </CardHeader>
            <CardContent>
              <Skeleton className="h-[220px] w-full" />
            </CardContent>
          </Card>
        ))}
      </div>
    )
  }

  if (summaries.length === 0) return null

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
      <RequestsByConnectionChart data={requestData} />
      <TokenUsageChart data={tokenData} />
      <SuccessRateChart data={successData} />
    </div>
  )
}

function RequestsByConnectionChart({ data }: { data: { name: string; requests: number; provider: string }[] }) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-semibold flex items-center gap-1.5">
          <Zap className="size-3.5 text-primary" />
          Requests by Connection
        </CardTitle>
      </CardHeader>
      <CardContent className="pt-0">
        <ResponsiveContainer width="100%" height={220}>
          <BarChart data={data} margin={{ top: 5, right: 5, left: -15, bottom: 5 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
            <XAxis
              dataKey="name"
              tick={{ fontSize: 10, fill: 'hsl(var(--muted-foreground))' }}
              interval={0}
              angle={-25}
              textAnchor="end"
              height={50}
            />
            <YAxis tick={{ fontSize: 10, fill: 'hsl(var(--muted-foreground))' }} />
            <RechartsTooltip contentStyle={tooltipStyle} />
            <Bar dataKey="requests" radius={[4, 4, 0, 0]}>
              {data.map((entry, i) => (
                <Cell key={i} fill={PROVIDER_COLORS[entry.provider] || '#6b7280'} fillOpacity={0.85} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  )
}

function TokenUsageChart({ data }: { data: { name: string; input: number; output: number; total: number }[] }) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-semibold flex items-center gap-1.5">
          <Coins className="size-3.5 text-primary" />
          Token Usage
          <div className="flex gap-2 ml-auto">
            <Badge variant="outline" className="text-[10px] px-1 py-0">
              <span className="inline-block size-2 rounded-sm bg-blue-500 mr-1" /> In
            </Badge>
            <Badge variant="outline" className="text-[10px] px-1 py-0">
              <span className="inline-block size-2 rounded-sm bg-orange-500 mr-1" /> Out
            </Badge>
          </div>
        </CardTitle>
      </CardHeader>
      <CardContent className="pt-0">
        <ResponsiveContainer width="100%" height={220}>
          <BarChart data={data} margin={{ top: 5, right: 5, left: -15, bottom: 5 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
            <XAxis
              dataKey="name"
              tick={{ fontSize: 10, fill: 'hsl(var(--muted-foreground))' }}
              interval={0}
              angle={-25}
              textAnchor="end"
              height={50}
            />
            <YAxis tick={{ fontSize: 10, fill: 'hsl(var(--muted-foreground))' }} tickFormatter={v => formatTokens(v)} />
            <RechartsTooltip
              contentStyle={tooltipStyle}
              formatter={(value: number, name: string) => [formatTokens(value), name === 'input' ? 'Input' : 'Output']}
            />
            <Bar dataKey="input" stackId="a" fill="#3b82f6" name="input" radius={[0, 0, 0, 0]} />
            <Bar dataKey="output" stackId="a" fill="#f97316" name="output" radius={[4, 4, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  )
}

function SuccessRateChart({ data }: { data: { name: string; rate: number; errors: number; total: number }[] }) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-semibold flex items-center gap-1.5">
          <CheckCircle2 className="size-3.5 text-primary" />
          Success Rate
        </CardTitle>
      </CardHeader>
      <CardContent className="pt-0">
        <ResponsiveContainer width="100%" height={220}>
          <BarChart data={data} margin={{ top: 5, right: 5, left: -15, bottom: 5 }} layout="vertical">
            <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
            <XAxis type="number" domain={[0, 100]} tick={{ fontSize: 10, fill: 'hsl(var(--muted-foreground))' }} tickFormatter={v => `${v}%`} />
            <YAxis
              type="category"
              dataKey="name"
              tick={{ fontSize: 10, fill: 'hsl(var(--muted-foreground))' }}
              width={80}
            />
            <RechartsTooltip
              contentStyle={tooltipStyle}
              formatter={(value: number, _name: string, props: any) => {
                const d = props.payload
                return [`${value}% (${d.errors} err / ${d.total} total)`, 'Success']
              }}
            />
            <Bar dataKey="rate" radius={[0, 4, 4, 0]} barSize={16}>
              {data.map((entry, i) => (
                <Cell
                  key={i}
                  fill={
                    entry.rate >= 95 ? '#10b981' :
                    entry.rate >= 80 ? '#f59e0b' :
                    '#ef4444'
                  }
                  fillOpacity={0.85}
                />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  )
}
