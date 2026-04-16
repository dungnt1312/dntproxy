import { useEffect, useState, useCallback, useRef } from 'react'
import { motion } from 'framer-motion'
import { CheckCircle2, AlertTriangle, Activity, Zap, ArrowRight } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ProviderLogo } from '@/components/connections/ProviderLogo'
import { cn } from '@/lib/utils'
import { goApi } from '@/lib/go-api'
import type { LogConnectionSummary } from '@/types/logs'

function calculateConnStatus(
  lastUsedMs: number,
  rateLimitedUntil: string | null,
  lastError: string | null,
): string {
  if (rateLimitedUntil && new Date(rateLimitedUntil).getTime() > Date.now()) return 'rate_limited'
  if (lastError) return 'error'
  if (!lastUsedMs) return 'idle'
  const age = Date.now() - lastUsedMs
  if (age < 30_000) return 'active'
  if (age < 300_000) return 'recent'
  return 'idle'
}

const DOT: Record<string, { c: string; pulse: boolean }> = {
  active: { c: 'bg-emerald-500', pulse: true },
  recent: { c: 'bg-amber-500', pulse: false },
  idle: { c: 'bg-gray-400 dark:bg-gray-600', pulse: false },
  error: { c: 'bg-red-500', pulse: false },
  rate_limited: { c: 'bg-orange-500', pulse: false },
}

export function DashboardHeader({ navigate }: { navigate: (path: string) => void }) {
  const [status, setStatus] = useState<{
    healthy: boolean | null
    errorCount: number
    activeConns: { id: string; name: string; provider: string; status: string; lastUsedMs: number }[]
  }>({ healthy: null, errorCount: 0, activeConns: [] })

  const intervalRef = useRef<ReturnType<typeof setInterval>>(undefined)

  const fetch = useCallback(async () => {
    try {
      const [conns, summaries, logSummary] = await Promise.all([
        goApi.getConnections().catch(() => []),
        goApi.getLogConnections({ range: '1h' }).catch((): LogConnectionSummary[] => []),
        goApi.getLogSummary().catch(() => null),
      ])
      const connArr = Array.isArray(conns) ? conns : []
      const summArr = Array.isArray(summaries) ? summaries : []
      const summMap = new Map(summArr.map(s => [s.connectionId, s]))

      const activeConns = connArr.map((c: any) => {
        const s = summMap.get(c.id)
        return {
          id: c.id,
          name: c.name,
          provider: c.provider,
          status: calculateConnStatus(s?.lastUsedMs || 0, c.backoffUntil || null, c.lastError || null),
          lastUsedMs: s?.lastUsedMs || 0,
        }
      }).filter(c => c.status === 'active' || c.status === 'recent')

      setStatus({
        healthy: (logSummary?.errorRequests || 0) === 0,
        errorCount: logSummary?.errorRequests || 0,
        activeConns,
      })
    } catch {
      // silent
    }
  }, [])

  useEffect(() => {
    fetch()
    intervalRef.current = setInterval(fetch, 15_000)
    return () => { if (intervalRef.current) clearInterval(intervalRef.current) }
  }, [fetch])

  const Icon = status.healthy === false ? AlertTriangle : CheckCircle2
  const color = status.healthy === false ? 'text-amber-500' : 'text-emerald-500'

  return (
    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <div className="flex items-center gap-3">
        <Icon className={cn('size-5', color)} />
        <div>
          <h1 className="text-xl font-bold leading-tight">Dashboard</h1>
          <div className="flex items-center gap-2 mt-0.5 flex-wrap">
            {status.errorCount > 0 && (
              <Badge variant="destructive" className="text-[10px] gap-1 px-1.5 py-0">
                {status.errorCount} errors
              </Badge>
            )}
            {status.activeConns.slice(0, 3).map(c => {
              const d = DOT[c.status]
              return (
                <span key={c.id} className="flex items-center gap-1 text-xs text-muted-foreground">
                  <ProviderLogo provider={c.provider} size={12} />
                  <span className="max-w-[60px] truncate">{c.name}</span>
                  <span className={cn('size-1.5 rounded-full', d.c, d.pulse && 'animate-pulse')} />
                </span>
              )
            })}
            {status.activeConns.length > 3 && (
              <span className="text-xs text-muted-foreground">+{status.activeConns.length - 3}</span>
            )}
          </div>
        </div>
      </div>
      <Button variant="outline" size="sm" className="gap-1.5 shrink-0" onClick={() => navigate('/playground')}>
        <Zap className="size-3.5" />
        Playground
        <ArrowRight className="size-3" />
      </Button>
    </div>
  )
}
