import { useEffect, useState, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  CheckCircle2,
  AlertTriangle,
  Radio,
  Pause,
  Play,
  X,
  Zap,
  ArrowRight,
  ChevronRight,
} from 'lucide-react'
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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { ProviderLogo } from '@/components/connections/ProviderLogo'
import { cn } from '@/lib/utils'
import { goApi } from '@/lib/go-api'
import { getStoredApiKey } from '@/lib/go-api'
import type { DailyUsageStat, LogEntry } from '@/types/logs'

// ─── Types ───────────────────────────────────────────────────────────────────

type KpiRange = '24h' | '7d' | '30d'

interface KpiData {
  totalRequests: number
  errorRequests: number
  totalTokens: number
  costTotal: number
  avgLatencyMs: number
}

interface RecentError {
  id: string
  createdAt: string
  errorMessage: string | null
  responseStatus: number
  provider: string | null
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

function formatCost(n: number): string {
  if (n >= 1) return `$${n.toFixed(2)}`
  if (n > 0) return `$${n.toFixed(4)}`
  return '$0'
}

function formatLatency(ms: number): string {
  if (!ms) return '—'
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

function formatRelativeTime(dateStr: string | null): string {
  if (!dateStr) return ''
  const diffMs = Date.now() - new Date(dateStr).getTime()
  const sec = Math.floor(diffMs / 1000)
  if (sec < 60) return `${sec}s ago`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.floor(min / 60)
  return `${hr}h ago`
}

function formatDayLabel(date: string): string {
  const d = new Date(date + 'T00:00:00')
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

// Map range → days count for chart
const RANGE_DAYS: Record<KpiRange, string> = { '24h': '7d', '7d': '7d', '30d': '30d' }
const KPI_RANGE_LABELS: Record<KpiRange, string> = { '24h': 'Today', '7d': '7 days', '30d': '30 days' }

// ─── Main Dashboard ──────────────────────────────────────────────────────────

export default function DashboardScreen() {
  const navigate = useNavigate()
  const [kpiRange, setKpiRange] = useState<KpiRange>('24h')
  const [kpi, setKpi] = useState<KpiData | null>(null)
  const [recentErrors, setRecentErrors] = useState<RecentError[]>([])
  const [dailyStats, setDailyStats] = useState<DailyUsageStat[]>([])
  const [loading, setLoading] = useState(true)
  const [chartLoading, setChartLoading] = useState(true)
  const kpiIntervalRef = useRef<ReturnType<typeof setInterval>>(undefined)
  const chartIntervalRef = useRef<ReturnType<typeof setInterval>>(undefined)

  const fetchKpi = useCallback(async (range: KpiRange) => {
    try {
      const summary = await goApi.getLogSummary({ range }).catch(() => null)
      setKpi({
        totalRequests: summary?.totalRequests ?? 0,
        errorRequests: summary?.errorRequests ?? 0,
        totalTokens: (summary?.inputTokens ?? 0) + (summary?.outputTokens ?? 0),
        costTotal: summary?.costTotal ?? 0,
        avgLatencyMs: summary?.avgLatencyMs ?? 0,
      })
      setRecentErrors(summary?.recentErrors ?? [])
    } finally {
      setLoading(false)
    }
  }, [])

  const fetchChart = useCallback(async (range: KpiRange) => {
    try {
      const chartRange = RANGE_DAYS[range]
      const stats = await goApi.getLogDaily(chartRange).catch(() => [])
      setDailyStats(Array.isArray(stats) ? stats : [])
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
                : kpi === null
                  ? 'No data'
                  : isHealthy
                    ? 'System Healthy'
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
                  'px-2.5 py-1 text-xs rounded font-medium transition-colors',
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

      {/* ── KPI Cards (4 cards: Requests / Success% / Total Tokens / Avg Latency / Cost) ── */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
        {loading ? (
          Array.from({ length: 5 }).map((_, i) => (
            <Card key={i}><CardContent className="px-4 py-3"><Skeleton className="h-12 w-full" /></CardContent></Card>
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
            <StatCard label="Est. Cost" value={`~${formatCost(kpi?.costTotal ?? 0)}`} accent="text-amber-500" range={kpiRange} />
          </>
        )}
      </div>

      {/* ── Chart + Live Feed ── */}
      <div className="grid grid-cols-1 lg:grid-cols-5 gap-4">
        <div className="lg:col-span-3 space-y-3">
          <DailyUsageChart data={dailyStats} loading={chartLoading} range={kpiRange} />
          {recentErrors.length > 0 && (
            <ErrorsHint errors={recentErrors} onViewAll={() => navigate('/logs?level=ERROR')} />
          )}
        </div>

        <div className="lg:col-span-2">
          <LiveFeed />
        </div>
      </div>
    </div>
  )
}

// ─── StatCard ─────────────────────────────────────────────────────────────────

function StatCard({ label, value, accent, range }: {
  label: string
  value: string
  accent?: string
  range: KpiRange
}) {
  return (
    <Card>
      <CardContent className="px-4 py-3 flex flex-col gap-1">
        <span className="text-[10px] text-muted-foreground uppercase font-semibold tracking-wider">{label}</span>
        <span className={cn('text-2xl font-bold tabular-nums leading-none', accent ?? 'text-foreground')}>{value}</span>
        <span className="text-[9px] text-muted-foreground/60">{KPI_RANGE_LABELS[range]}</span>
      </CardContent>
    </Card>
  )
}

// ─── Daily Usage Chart ───────────────────────────────────────────────────────

const CHART_COLORS = {
  requests: '#60a5fa',
  errors: '#f87171',
  tokens: '#34d399',
}

const CHART_RANGE_LABELS: Record<KpiRange, string> = {
  '24h': 'Last 7 days',
  '7d': 'Last 7 days',
  '30d': 'Last 30 days',
}

function DailyUsageChart({ data, loading, range }: { data: DailyUsageStat[]; loading: boolean; range: KpiRange }) {
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
          <span className="text-[10px] text-muted-foreground uppercase tracking-wider">{CHART_RANGE_LABELS[range]}</span>
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
            <ComposedChart data={chartData} margin={{ top: 4, right: 8, left: -8, bottom: 0 }}>
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
                  background: '#1e2329',
                  border: '1px solid rgba(255,255,255,0.12)',
                  borderRadius: '8px',
                  fontSize: 12,
                  color: '#f1f5f9',
                }}
                labelStyle={{ fontWeight: 600, marginBottom: 4, color: '#94a3b8' }}
                cursor={{ fill: 'rgba(255,255,255,0.04)' }}
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

// ─── Errors Hint ──────────────────────────────────────────────────────────────

function ErrorsHint({ errors, onViewAll }: { errors: RecentError[]; onViewAll: () => void }) {
  return (
    <Card className="border-red-500/20 bg-red-500/5">
      <CardContent className="px-4 py-3">
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-semibold text-red-400 flex items-center gap-1.5">
            <AlertTriangle className="size-3.5" />
            Recent Errors
            <Badge variant="destructive" className="text-[9px] px-1.5 py-0 ml-0.5">{errors.length}</Badge>
          </span>
          <button
            onClick={onViewAll}
            className="text-[10px] text-muted-foreground hover:text-foreground flex items-center gap-0.5 transition-colors"
          >
            View all <ChevronRight className="size-3" />
          </button>
        </div>
        <div className="space-y-1.5">
          {errors.slice(0, 3).map(err => (
            <div key={err.id} className="flex items-start gap-2 text-xs">
              <Badge variant="destructive" className="text-[9px] px-1.5 py-0 shrink-0 mt-0.5">
                {err.responseStatus || 'ERR'}
              </Badge>
              <span className="text-red-300/90 truncate flex-1">
                {err.errorMessage || 'Unknown error'}
              </span>
              <span className="text-muted-foreground shrink-0 whitespace-nowrap">
                {formatRelativeTime(err.createdAt)}
              </span>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

// ─── Live Feed (SSE) ──────────────────────────────────────────────────────────

const SSE_BASE = import.meta.env.VITE_GO_API_URL || '/api'
const MAX_ENTRIES = 50

function LiveFeed() {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [paused, setPaused] = useState(false)
  const esRef = useRef<EventSource | null>(null)
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pausedRef = useRef(false)

  useEffect(() => { pausedRef.current = paused }, [paused])

  const clearReconnect = useCallback(() => {
    if (reconnectRef.current) { clearTimeout(reconnectRef.current); reconnectRef.current = null }
  }, [])

  const connect = useCallback(() => {
    clearReconnect()
    if (esRef.current) { esRef.current.close(); esRef.current = null }

    const keyParam = getStoredApiKey() ? `&key=${encodeURIComponent(getStoredApiKey())}` : ''
    const es = new EventSource(`${SSE_BASE}/logs/stream?range=1h&limit=${MAX_ENTRIES}${keyParam}`)
    esRef.current = es

    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        if (data.type === 'init' && Array.isArray(data.logs)) {
          setEntries(data.logs.slice(0, MAX_ENTRIES))
        } else if (data.type === 'delta' && data.log) {
          setEntries(prev => {
            if (prev.some(entry => entry.id === data.log.id)) return prev
            return [data.log, ...prev].slice(0, MAX_ENTRIES)
          })
        }
      } catch { /* ignore */ }
    }

    es.onerror = () => {
      es.close(); esRef.current = null; clearReconnect()
      reconnectRef.current = setTimeout(() => {
        reconnectRef.current = null
        if (!pausedRef.current) connect()
      }, 5000)
    }
  }, [clearReconnect])

  useEffect(() => {
    if (!paused) connect()
    else if (esRef.current) { esRef.current.close(); esRef.current = null }
    return () => { clearReconnect(); if (esRef.current) { esRef.current.close(); esRef.current = null } }
  }, [paused, connect, clearReconnect])

  return (
    <Card className="flex flex-col overflow-hidden" style={{ minHeight: 320 }}>
      <CardHeader className="pb-2 shrink-0">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold flex items-center gap-1.5">
            <Radio className={cn('size-3.5', !paused && entries.length > 0 && 'text-emerald-500 animate-pulse')} />
            Live Requests
            {entries.length > 0 && (
              <Badge variant="secondary" className="text-[9px] px-1.5 py-0">{entries.length}</Badge>
            )}
          </h2>
          <div className="flex gap-1">
            <Button variant="ghost" size="icon" className="size-7" title={paused ? 'Resume' : 'Pause'} onClick={() => setPaused(p => !p)}>
              {paused ? <Play className="size-3" /> : <Pause className="size-3" />}
            </Button>
            <Button variant="ghost" size="icon" className="size-7" title="Clear" onClick={() => setEntries([])}>
              <X className="size-3" />
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="pt-0 flex-1 overflow-hidden">
        {entries.length === 0 ? (
          <div className="flex items-center justify-center h-full text-muted-foreground text-sm py-10">
            {paused ? 'Stream paused' : 'Waiting for requests...'}
          </div>
        ) : (
          <ScrollArea className="h-[calc(100vh-360px)] min-h-[260px]">
            <table className="w-full text-xs border-collapse">
              <thead className="sticky top-0 bg-card z-10">
                <tr className="border-b border-border">
                  <th className="py-1.5 w-2"></th>
                  <th className="py-1.5 text-left font-semibold text-muted-foreground text-[10px] uppercase tracking-wider">Model</th>
                  <th className="py-1.5 text-left font-semibold text-muted-foreground text-[10px] uppercase tracking-wider">Conn</th>
                  <th className="py-1.5 text-right font-semibold text-muted-foreground text-[10px] uppercase tracking-wider whitespace-nowrap">Tokens</th>
                  <th className="py-1.5 text-right font-semibold text-muted-foreground text-[10px] uppercase tracking-wider">Dur</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/30">
                {entries.map(entry => <FeedRow key={entry.id} entry={entry} />)}
              </tbody>
            </table>
          </ScrollArea>
        )}
      </CardContent>
    </Card>
  )
}

function FeedRow({ entry }: { entry: LogEntry }) {
  const isError = entry.level === 'ERROR' || (entry.statusCode && entry.statusCode >= 400)

  return (
    <tr className={cn('transition-colors', isError ? 'bg-red-50/50 dark:bg-red-950/10' : 'hover:bg-muted/30')}>
      <td className="py-1.5 pl-1">
        <span className={cn('block w-1.5 h-1.5 rounded-full', isError ? 'bg-red-500' : 'bg-emerald-500')} />
      </td>
      <td className="py-1.5 font-mono truncate max-w-[90px]" title={entry.model}>
        {entry.model?.split('/').pop() || entry.path || '-'}
      </td>
      <td className="py-1.5">
        <div className="flex items-center gap-1">
          {entry.provider && <ProviderLogo provider={entry.provider} size={12} />}
          <span className="truncate max-w-[55px] text-muted-foreground text-[10px]">{entry.connectionName || '-'}</span>
        </div>
      </td>
      <td className="py-1.5 text-right whitespace-nowrap tabular-nums">
        {(entry.totalTokens || 0) > 0
          ? <span className="text-emerald-400">{formatTokens(entry.totalTokens || 0)}</span>
          : <span className="text-muted-foreground/40">—</span>}
      </td>
      <td className="py-1.5 text-right text-muted-foreground whitespace-nowrap font-mono">
        {entry.durationMs
          ? (entry.durationMs < 1000 ? `${entry.durationMs}ms` : `${(entry.durationMs / 1000).toFixed(1)}s`)
          : <span className="text-muted-foreground/40">{formatRelativeTime(entry.timestamp)}</span>}
      </td>
    </tr>
  )
}
