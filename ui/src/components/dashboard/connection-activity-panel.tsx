import { useEffect, useState, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { RefreshCw, Activity } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { ConnectionActivityCard, type ConnectionStatus } from './connection-activity-card'
import { goApi } from '@/lib/go-api'
import type { LogConnectionSummary } from '@/types/logs'

interface EnrichedConnection {
  id: string
  name: string
  provider: string
  status: ConnectionStatus
  summary: LogConnectionSummary
  rateLimitedUntil: string | null
  lastError: string | null
}

const STATUS_ORDER: Record<ConnectionStatus, number> = {
  active: 0,
  recent: 1,
  rate_limited: 2,
  error: 3,
  idle: 4,
}

function calculateStatus(
  lastUsedMs: number,
  rateLimitedUntil: string | null,
  lastError: string | null,
): ConnectionStatus {
  if (rateLimitedUntil && new Date(rateLimitedUntil).getTime() > Date.now()) return 'rate_limited'
  if (lastError) return 'error'
  if (!lastUsedMs) return 'idle'
  const age = Date.now() - lastUsedMs
  if (age < 30_000) return 'active'
  if (age < 300_000) return 'recent'
  return 'idle'
}

const EMPTY_SUMMARY: LogConnectionSummary = {
  connectionId: '',
  connectionName: '',
  provider: '',
  requests: 0,
  errors: 0,
  totalTokens: 0,
  inputTokens: 0,
  outputTokens: 0,
  costTotal: 0,
  currency: 'USD',
  lastUsedMs: 0,
  avgLatencyMs: 0,
}

export function ConnectionActivityPanel() {
  const navigate = useNavigate()
  const [connections, setConnections] = useState<EnrichedConnection[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const intervalRef = useRef<ReturnType<typeof setInterval>>(undefined)

  const fetchData = useCallback(async () => {
    try {
      const [conns, summaries] = await Promise.all([
        goApi.getConnections().catch(() => []),
        goApi.getLogConnections({ range: '1h' }).catch((): LogConnectionSummary[] => []),
      ])

      const summaryMap = new Map<string, LogConnectionSummary>()
      for (const s of Array.isArray(summaries) ? summaries : []) {
        summaryMap.set(s.connectionId, s)
      }

      const enriched: EnrichedConnection[] = (Array.isArray(conns) ? conns : []).map((c: any) => {
        const summary = summaryMap.get(c.id) || { ...EMPTY_SUMMARY, connectionId: c.id, provider: c.provider }
        const status = calculateStatus(summary.lastUsedMs, c.backoffUntil || null, c.lastError || null)
        return {
          id: c.id,
          name: c.name,
          provider: c.provider,
          status,
          summary,
          rateLimitedUntil: c.backoffUntil || null,
          lastError: c.lastError || null,
        }
      })

      enriched.sort((a, b) => STATUS_ORDER[a.status] - STATUS_ORDER[b.status])
      setConnections(enriched)
    } catch {
      // silent
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
    intervalRef.current = setInterval(fetchData, 30_000)
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current)
    }
  }, [fetchData])

  const handleRefresh = () => {
    setRefreshing(true)
    fetchData()
  }

  const activeCount = connections.filter(c => c.status === 'active' || c.status === 'recent').length

  return (
    <Card className="border-2 border-primary/10">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-base font-semibold">
            <Activity className="size-4 text-primary" />
            Connections
            <span className="text-xs font-normal text-muted-foreground">
              {activeCount > 0 ? `${activeCount} active` : 'No active'}
            </span>
          </CardTitle>
          <Button
            variant="ghost"
            size="icon"
            className="size-7"
            onClick={handleRefresh}
            disabled={refreshing}
          >
            <RefreshCw className={cn('size-3.5', refreshing && 'animate-spin')} />
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="h-36 rounded-lg bg-muted animate-pulse" />
            ))}
          </div>
        ) : connections.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground text-sm">
            No connections configured.{' '}
            <span
              className="underline cursor-pointer hover:text-foreground"
              onClick={() => navigate('/connections')}
            >
              Add one
            </span>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
            {connections.map(conn => (
              <ConnectionActivityCard
                key={conn.id}
                id={conn.id}
                name={conn.name}
                provider={conn.provider}
                status={conn.status}
                summary={conn.summary}
                rateLimitedUntil={conn.rateLimitedUntil}
                lastError={conn.lastError}
                onClick={() => navigate(`/logs?connectionId=${conn.id}`)}
              />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function cn(...classes: (string | boolean | undefined)[]) {
  return classes.filter(Boolean).join(' ')
}
