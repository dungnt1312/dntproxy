import { useEffect, useState, useCallback, useRef, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { Activity, CheckCircle2, AlertTriangle, Zap, ArrowRight } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ProviderLogo } from '@/components/connections/ProviderLogo'
import { cn } from '@/lib/utils'
import { goApi } from '@/lib/go-api'
import type { LogConnectionSummary } from '@/types/logs'

interface HeroData {
  totalRequests: number
  successRate: number
  activeConnections: number
  totalConnections: number
  hasErrors: boolean
  errorCount: number
  topConnections: {
    id: string
    name: string
    provider: string
    status: 'active' | 'recent' | 'idle' | 'error' | 'rate_limited'
    requests: number
    lastUsedMs: number
  }[]
}

function calculateConnStatus(
  lastUsedMs: number,
  rateLimitedUntil: string | null,
  lastError: string | null,
): HeroData['topConnections'][0]['status'] {
  if (rateLimitedUntil && new Date(rateLimitedUntil).getTime() > Date.now()) return 'rate_limited'
  if (lastError) return 'error'
  if (!lastUsedMs) return 'idle'
  const age = Date.now() - lastUsedMs
  if (age < 30_000) return 'active'
  if (age < 300_000) return 'recent'
  return 'idle'
}

function formatLastUsed(ms: number): string {
  if (!ms) return ''
  const diff = Date.now() - ms
  if (diff < 0) return 'now'
  const sec = Math.floor(diff / 1000)
  if (sec < 5) return 'now'
  if (sec < 60) return `${sec}s`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m`
  return `${Math.floor(min / 60)}h`
}

const STATUS_DOT: Record<string, { color: string; pulse: boolean }> = {
  active: { color: 'bg-emerald-500', pulse: true },
  recent: { color: 'bg-amber-500', pulse: false },
  idle: { color: 'bg-gray-400 dark:bg-gray-600', pulse: false },
  error: { color: 'bg-red-500', pulse: false },
  rate_limited: { color: 'bg-orange-500', pulse: false },
}

export function HeroSection() {
  const navigate = useNavigate()
  const [data, setData] = useState<HeroData | null>(null)
  const [loading, setLoading] = useState(true)
  const intervalRef = useRef<ReturnType<typeof setInterval>>(undefined)

  const fetchData = useCallback(async () => {
    try {
      const [conns, summaries, logSummary] = await Promise.all([
        goApi.getConnections().catch(() => []),
        goApi.getLogConnections({ range: '1h' }).catch((): LogConnectionSummary[] => []),
        goApi.getLogSummary().catch(() => null),
      ])

      const connArr = Array.isArray(conns) ? conns : []
      const summArr = Array.isArray(summaries) ? summaries : []
      const summMap = new Map(summArr.map(s => [s.connectionId, s]))

      const totalReq = logSummary?.totalRequests || 0
      const errReq = logSummary?.errorRequests || 0
      const successRate = totalReq > 0 ? ((totalReq - errReq) / totalReq) * 100 : 100

      const topConnections = connArr.slice(0, 5).map((c: any) => {
        const s = summMap.get(c.id)
        return {
          id: c.id,
          name: c.name,
          provider: c.provider,
          status: calculateConnStatus(s?.lastUsedMs || 0, c.backoffUntil || null, c.lastError || null),
          requests: s?.requests || 0,
          lastUsedMs: s?.lastUsedMs || 0,
        }
      }).sort((a, b) => {
        const order = { active: 0, recent: 1, rate_limited: 2, error: 3, idle: 4 }
        return (order[a.status] ?? 4) - (order[b.status] ?? 4)
      })

      setData({
        totalRequests: totalReq,
        successRate,
        activeConnections: connArr.filter((c: any) => {
          const s = summMap.get(c.id)
          const st = calculateConnStatus(s?.lastUsedMs || 0, c.backoffUntil || null, c.lastError || null)
          return st === 'active' || st === 'recent'
        }).length,
        totalConnections: connArr.length,
        hasErrors: errReq > 0,
        errorCount: errReq,
        topConnections,
      })
    } catch {
      // silent
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
    intervalRef.current = setInterval(fetchData, 15_000)
    return () => { if (intervalRef.current) clearInterval(intervalRef.current) }
  }, [fetchData])

  const healthStatus = useMemo(() => {
    if (!data) return { label: 'Loading...', color: 'text-muted-foreground', icon: <Activity className="size-4" /> }
    if (data.hasErrors && data.successRate < 80) return { label: 'Issues Detected', color: 'text-red-500', icon: <AlertTriangle className="size-4" /> }
    if (data.hasErrors) return { label: 'Degraded', color: 'text-amber-500', icon: <AlertTriangle className="size-4" /> }
    if (data.totalConnections === 0) return { label: 'No Connections', color: 'text-muted-foreground', icon: <Activity className="size-4" /> }
    return { label: 'Healthy', color: 'text-emerald-500', icon: <CheckCircle2 className="size-4" /> }
  }, [data])

  if (loading) {
    return (
      <Card className="border-2 border-primary/10 overflow-hidden">
        <CardContent className="p-6">
          <div className="animate-pulse space-y-4">
            <div className="flex items-center gap-3">
              <div className="size-10 rounded-xl bg-muted" />
              <div className="space-y-2 flex-1">
                <div className="h-5 w-40 bg-muted rounded" />
                <div className="h-3 w-64 bg-muted rounded" />
              </div>
            </div>
            <div className="flex gap-6">
              <div className="h-8 w-20 bg-muted rounded" />
              <div className="h-8 w-20 bg-muted rounded" />
              <div className="h-8 w-20 bg-muted rounded" />
            </div>
          </div>
        </CardContent>
      </Card>
    )
  }

  if (!data) return null

  const activeConns = data.topConnections.filter(c => c.status === 'active' || c.status === 'recent')

  return (
    <Card className={cn(
      'border-2 overflow-hidden',
      healthStatus.label === 'Healthy' && 'border-emerald-200 dark:border-emerald-800/50',
      healthStatus.label === 'Degraded' && 'border-amber-200 dark:border-amber-800/50',
      healthStatus.label === 'Issues Detected' && 'border-red-200 dark:border-red-800/50',
    )}>
      <CardContent className="p-5 sm:p-6">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className={cn(
              'size-10 rounded-xl flex items-center justify-center',
              healthStatus.label === 'Healthy' && 'bg-emerald-100 dark:bg-emerald-900/30',
              healthStatus.label === 'Degraded' && 'bg-amber-100 dark:bg-amber-900/30',
              healthStatus.label === 'Issues Detected' && 'bg-red-100 dark:bg-red-900/30',
              healthStatus.label === 'No Connections' && 'bg-muted',
            )}>
              {healthStatus.icon}
            </div>
            <div>
              <h2 className={cn('text-lg font-bold', healthStatus.color)}>
                System {healthStatus.label}
              </h2>
              <p className="text-xs text-muted-foreground mt-0.5">
                {data.activeConnections > 0
                  ? `${data.activeConnections} active connection${data.activeConnections > 1 ? 's' : ''} · ${data.totalRequests.toLocaleString()} requests (1h)`
                  : 'No active connections'
                }
              </p>
            </div>
          </div>

          <Button
            variant="outline"
            size="sm"
            className="gap-1.5 shrink-0"
            onClick={() => navigate('/playground')}
          >
            <Zap className="size-3.5" />
            Test Request
            <ArrowRight className="size-3" />
          </Button>
        </div>

        <div className="mt-4 flex flex-col sm:flex-row sm:items-center gap-4 sm:gap-6">
          <div className="flex items-baseline gap-1.5">
            <span className="text-2xl font-bold tabular-nums">{data.successRate.toFixed(1)}%</span>
            <span className="text-xs text-muted-foreground">success</span>
          </div>

          <div className="hidden sm:block size-px h-8 bg-border" />

          <div className="flex items-baseline gap-1.5">
            <span className="text-2xl font-bold tabular-nums">
              {data.activeConnections}<span className="text-base text-muted-foreground font-normal">/{data.totalConnections}</span>
            </span>
            <span className="text-xs text-muted-foreground">connections</span>
          </div>

          <div className="hidden sm:block size-px h-8 bg-border" />

          {data.hasErrors && (
            <>
              <Badge variant="destructive" className="text-xs gap-1">
                <AlertTriangle className="size-3" />
                {data.errorCount} error{data.errorCount > 1 ? 's' : ''}
              </Badge>
              <div className="hidden sm:block size-px h-8 bg-border" />
            </>
          )}

          {activeConns.length > 0 && (
            <div className="flex items-center gap-2 overflow-hidden">
              {activeConns.slice(0, 4).map(conn => {
                const dot = STATUS_DOT[conn.status]
                return (
                  <div key={conn.id} className="flex items-center gap-1.5 shrink-0">
                    <div className="relative shrink-0">
                      <ProviderLogo provider={conn.provider} size={16} />
                      <div className={cn(
                        'absolute -bottom-0.5 -right-0.5 size-2 rounded-full border border-white dark:border-gray-950',
                        dot.color,
                        dot.pulse && 'animate-pulse',
                      )} />
                    </div>
                    <span className="text-xs font-medium truncate max-w-[80px]">{conn.name}</span>
                    <span className="text-[10px] text-muted-foreground tabular-nums">
                      {formatLastUsed(conn.lastUsedMs)}
                    </span>
                  </div>
                )
              })}
              {activeConns.length > 4 && (
                <span className="text-xs text-muted-foreground">+{activeConns.length - 4}</span>
              )}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
