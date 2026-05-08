import { useEffect, useState, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { CheckCircle2, AlertTriangle, Zap, ArrowRight } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { goApi } from '@/lib/go-api'
import { StatCard } from './dashboard/stat-card'
import { DailyUsageChart } from './dashboard/daily-usage-chart'
import { ErrorsHint, type RecentError } from './dashboard/errors-hint'
import { LiveFeed } from './dashboard/live-feed'
import {
  formatTokens,
  formatCost,
  formatLatency,
  formatRelativeTime,
  KPI_RANGE_LABELS,
  type KpiRange,
} from './dashboard/helpers'
import type { DailyUsageStat } from '@/types/logs'

// ─── Types ───────────────────────────────────────────────────────────────────

interface KpiData {
  totalRequests: number
  errorRequests: number
  totalTokens: number
  costTotal: number
  avgLatencyMs: number
}

// ─── Main Dashboard ──────────────────────────────────────────────────────────

export default function DashboardScreen() {
  const navigate = useNavigate()
  const [kpiRange, setKpiRange] = useState<KpiRange>('24h')
  const [kpi, setKpi] = useState<KpiData | null>(null)
  const [recentErrors, setRecentErrors] = useState<RecentError[]>([])
  const [dailyStats, setDailyStats] = useState<DailyUsageStat[]>([])
  const [loading, setLoading] = useState(true)
  const [chartLoading, setChartLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [chartError, setChartError] = useState<string | null>(null)
  const [lastRefreshed, setLastRefreshed] = useState<number>(Date.now())
  const kpiIntervalRef = useRef<ReturnType<typeof setInterval>>(undefined)
  const chartIntervalRef = useRef<ReturnType<typeof setInterval>>(undefined)

  const fetchKpi = useCallback(async (range: KpiRange) => {
    try {
      const summary = await goApi.getLogSummary({ range })
      setKpi({
        totalRequests: summary?.totalRequests ?? 0,
        errorRequests: summary?.errorRequests ?? 0,
        totalTokens: (summary?.inputTokens ?? 0) + (summary?.outputTokens ?? 0),
        costTotal: summary?.costTotal ?? 0,
        avgLatencyMs: summary?.avgLatencyMs ?? 0,
      })
      setRecentErrors(summary?.recentErrors ?? [])
      setError(null)
      setLastRefreshed(Date.now())
    } catch {
      setError('Failed to load KPI data')
    } finally {
      setLoading(false)
    }
  }, [])

  const fetchChart = useCallback(async (range: KpiRange) => {
    try {
      const stats = await goApi.getLogDaily(range)
      setDailyStats(Array.isArray(stats) ? stats : [])
      setChartError(null)
    } catch {
      setChartError('Failed to load chart data')
    } finally {
      setChartLoading(false)
    }
  }, [])

  // Refetch everything when range changes
  useEffect(() => {
    setLoading(true)
    setChartLoading(true)
    fetchKpi(kpiRange)
    fetchChart(kpiRange)

    // Auto-refresh KPI every 30s, chart every 60s
    if (kpiIntervalRef.current) clearInterval(kpiIntervalRef.current)
    if (chartIntervalRef.current) clearInterval(chartIntervalRef.current)
    kpiIntervalRef.current = setInterval(() => fetchKpi(kpiRange), 30_000)
    chartIntervalRef.current = setInterval(() => fetchChart(kpiRange), 60_000)

    return () => {
      if (kpiIntervalRef.current) clearInterval(kpiIntervalRef.current)
      if (chartIntervalRef.current) clearInterval(chartIntervalRef.current)
    }
  }, [kpiRange, fetchKpi, fetchChart])

  const successRate = kpi && kpi.totalRequests > 0
    ? (((kpi.totalRequests - kpi.errorRequests) / kpi.totalRequests) * 100).toFixed(1)
    : null

  // Only healthy once kpi loaded and no errors
  const isHealthy = kpi !== null && kpi.errorRequests === 0

  return (
    <div className="space-y-4">
      {/* ── Header ── */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          {loading ? (
            <Skeleton className="h-5 w-5 rounded-full" />
          ) : isHealthy ? (
            <CheckCircle2 className="size-5 text-emerald-500" />
          ) : (
            <AlertTriangle className="size-5 text-amber-500" />
          )}
          <div>
            <h1 className="text-xl font-bold leading-tight">Dashboard</h1>
            <p className="text-xs text-muted-foreground mt-0.5">
              {loading
                ? 'Loading...'
                : error
                  ? error
                  : kpi === null
                    ? 'No data'
                    : isHealthy
                      ? `System Healthy · Updated ${formatRelativeTime(new Date(lastRefreshed).toISOString())}`
                      : `${kpi.errorRequests} error${kpi.errorRequests > 1 ? 's' : ''} in ${KPI_RANGE_LABELS[kpiRange].toLowerCase()}`}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2 shrink-0">
          <div className="flex items-center rounded-md border border-border bg-muted/30 p-0.5 gap-0.5">
            {(['24h', '7d', '30d'] as KpiRange[]).map(r => (
              <button
                key={r}
                onClick={() => setKpiRange(r)}
                className={cn(
                  'px-2.5 py-1 text-xs rounded font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                  kpiRange === r
                    ? 'bg-background shadow-sm text-foreground'
                    : 'text-muted-foreground hover:text-foreground',
                )}
              >
                {KPI_RANGE_LABELS[r]}
              </button>
            ))}
          </div>

          <Button variant="outline" size="sm" className="gap-1.5" onClick={() => navigate('/playground')}>
            <Zap className="size-3.5" />
            Playground
            <ArrowRight className="size-3" />
          </Button>
        </div>
      </div>

      {/* ── KPI Cards (5 cards: Requests / Success% / Total Tokens / Avg Latency / Cost) ── */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
        {loading ? (
          Array.from({ length: 5 }).map((_, i) => (
            <Card key={i}>
              <CardContent className="px-4 py-3">
                <Skeleton className="h-12 w-full" />
              </CardContent>
            </Card>
          ))
        ) : (
          <>
            <StatCard label="Requests" value={(kpi?.totalRequests ?? 0).toLocaleString()} range={kpiRange} />
            <StatCard
              label="Success Rate"
              value={successRate ? `${successRate}%` : '—'}
              accent={
                successRate && parseFloat(successRate) >= 95
                  ? 'text-emerald-500'
                  : successRate && parseFloat(successRate) >= 80
                    ? 'text-amber-500'
                    : successRate
                      ? 'text-red-500'
                      : undefined
              }
              range={kpiRange}
            />
            <StatCard label="Total Tokens" value={formatTokens(kpi?.totalTokens ?? 0)} accent="text-blue-400" range={kpiRange} />
            <StatCard label="Avg Latency" value={formatLatency(kpi?.avgLatencyMs ?? 0)} range={kpiRange} />
            <StatCard label="Est. Cost" value={formatCost(kpi?.costTotal ?? 0)} accent="text-amber-500" range={kpiRange} />
          </>
        )}
      </div>

      {/* ── Chart ── */}
      <div className="space-y-3">
        <DailyUsageChart
          data={dailyStats}
          loading={chartLoading}
          error={chartError}
          range={kpiRange}
          onRetry={() => {
            setChartLoading(true)
            setChartError(null)
            fetchChart(kpiRange)
          }}
        />
        {recentErrors.length > 0 && (
          <ErrorsHint errors={recentErrors} onViewAll={() => navigate('/logs?level=ERROR')} />
        )}
      </div>

      {/* ── Live Feed ── */}
      <LiveFeed />
    </div>
  )
}
