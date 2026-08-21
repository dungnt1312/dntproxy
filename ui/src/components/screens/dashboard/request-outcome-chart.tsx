import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDayLabel } from './helpers'
import type { DailyUsageStat } from '@/types/logs'

interface RequestOutcomeChartProps {
  data: DailyUsageStat[]
  loading: boolean
}

export function RequestOutcomeChart({ data, loading }: RequestOutcomeChartProps) {
  const chartData = data.map(point => ({
    label: formatDayLabel(point.date),
    success: Math.max(0, point.requests - point.errors),
    failed: point.errors,
  }))
  const hasData = data.some(point => point.requests > 0)

  return <section className="rounded-xl border border-border/80 bg-card shadow-sm">
    <div className="flex items-center justify-between border-b border-border/70 px-4 py-3">
      <div><h2 className="text-sm font-semibold">Request outcomes</h2><p className="mt-0.5 text-[11px] text-muted-foreground">Completed requests by time period</p></div>
      <div className="flex items-center gap-3 text-[11px]"><span className="flex items-center gap-1.5 text-emerald-400"><i className="size-2 rounded-sm bg-emerald-400" />Success</span><span className="flex items-center gap-1.5 text-red-400"><i className="size-2 rounded-sm bg-red-400" />Failed</span></div>
    </div>
    <div className="px-3 pb-3 pt-4">
      {loading ? <Skeleton className="h-52 w-full rounded-lg" /> : !hasData ? <div className="flex h-52 items-center justify-center text-sm text-muted-foreground">No completed requests in this period</div> : <ResponsiveContainer width="100%" height={208}>
        <BarChart data={chartData} margin={{ top: 4, right: 4, left: -16, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.07)" vertical={false} />
          <XAxis dataKey="label" tick={{ fontSize: 10, fill: 'rgba(156,163,175,1)' }} tickLine={false} axisLine={false} interval={chartData.length > 14 ? 4 : chartData.length > 7 ? 1 : 0} />
          <YAxis allowDecimals={false} tick={{ fontSize: 10, fill: 'rgba(156,163,175,1)' }} tickLine={false} axisLine={false} width={38} />
          <Tooltip contentStyle={{ background: 'hsl(var(--popover))', border: '1px solid hsl(var(--border))', borderRadius: '8px', fontSize: 12, color: 'hsl(var(--popover-foreground))' }} labelStyle={{ fontWeight: 600, marginBottom: 4, color: 'hsl(var(--muted-foreground))' }} cursor={{ fill: 'hsl(var(--muted) / 0.3)' }} />
          <Legend wrapperStyle={{ display: 'none' }} />
          <Bar dataKey="success" name="Success" stackId="requests" fill="#34d399" radius={[0, 0, 0, 0]} maxBarSize={38} />
          <Bar dataKey="failed" name="Failed" stackId="requests" fill="#f87171" radius={[3, 3, 0, 0]} maxBarSize={38} />
        </BarChart>
      </ResponsiveContainer>}
    </div>
  </section>
}
