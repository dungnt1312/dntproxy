import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Activity, RefreshCw, Zap } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { goApi } from '@/lib/go-api'
import { StatCard } from './dashboard/stat-card'
import { RoutingTopology } from './dashboard/routing-topology'
import { RecentRequests } from './dashboard/recent-requests'
import { formatCost, formatRelativeTime, formatTokens } from './dashboard/helpers'
import type { Connection } from '@/types/connections'
import type { LogConnectionSummary, LogEntry } from '@/types/logs'
import type { AliasMap, ComboData } from '@/components/screens/routing/types'

type DashboardRange = '24h' | '7d' | '30d' | '60d'
type DashboardTab = 'overview' | 'details'

const RANGES: Array<{ value: DashboardRange; label: string; apiRange: '24h' | '7d' | '30d' }> = [
  { value: '24h', label: 'Today', apiRange: '24h' },
  { value: '7d', label: '7D', apiRange: '7d' },
  { value: '30d', label: '30D', apiRange: '30d' },
  { value: '60d', label: '60D', apiRange: '30d' },
]

interface Summary { totalRequests: number; inputTokens: number; outputTokens: number; costTotal: number; errorRequests: number }

export default function DashboardScreen() {
  const navigate = useNavigate()
  const [tab, setTab] = useState<DashboardTab>('overview')
  const [range, setRange] = useState<DashboardRange>('24h')
  const [summary, setSummary] = useState<Summary | null>(null)
  const [connections, setConnections] = useState<Connection[]>([])
  const [connectionSummaries, setConnectionSummaries] = useState<LogConnectionSummary[]>([])
  const [combos, setCombos] = useState<ComboData[]>([])
  const [aliases, setAliases] = useState<AliasMap>({})
  const [requests, setRequests] = useState<LogEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [lastRefreshed, setLastRefreshed] = useState(Date.now())
  const intervalRef = useRef<ReturnType<typeof setInterval>>(undefined)
  const apiRange = RANGES.find(item => item.value === range)?.apiRange ?? '24h'

  const fetchData = useCallback(async () => {
    const [nextSummary, nextConnections, nextConnectionSummaries, nextCombos, nextAliases, nextRequests] = await Promise.all([
      goApi.getLogSummary({ range: apiRange }).catch(() => null),
      goApi.getConnections().catch(() => []),
      goApi.getLogConnections({ range: apiRange }).catch(() => []),
      goApi.getCombos().catch(() => []),
      goApi.getAliases().catch(() => ({})),
      goApi.getLogs({ range: '24h', limit: 18 }).catch(() => []),
    ])
    if (nextSummary) setSummary({ totalRequests: nextSummary.totalRequests ?? 0, inputTokens: nextSummary.inputTokens ?? 0, outputTokens: nextSummary.outputTokens ?? 0, costTotal: nextSummary.costTotal ?? 0, errorRequests: nextSummary.errorRequests ?? 0 })
    setConnections(Array.isArray(nextConnections) ? nextConnections : [])
    setConnectionSummaries(Array.isArray(nextConnectionSummaries) ? nextConnectionSummaries : [])
    setCombos(Array.isArray(nextCombos) ? nextCombos : [])
    setAliases(nextAliases && typeof nextAliases === 'object' ? nextAliases : {})
    setRequests(Array.isArray(nextRequests) ? nextRequests : [])
    setLastRefreshed(Date.now())
    setLoading(false)
  }, [apiRange])

  useEffect(() => {
    setLoading(true)
    fetchData()
    if (intervalRef.current) clearInterval(intervalRef.current)
    intervalRef.current = setInterval(fetchData, 15_000)
    return () => { if (intervalRef.current) clearInterval(intervalRef.current) }
  }, [fetchData])

  return <div className="dashboard-console -mx-4 -mt-4 min-h-[calc(100vh-4rem)] px-4 py-5 sm:-mx-6 sm:px-6">
    <div className="mx-auto max-w-7xl space-y-5">
      <header className="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
        <div className="inline-flex w-fit rounded-xl border border-border/80 bg-card/90 p-1 shadow-sm">
          {(['overview', 'details'] as DashboardTab[]).map(item => <button key={item} onClick={() => setTab(item)} className={cn('rounded-lg px-4 py-2 text-sm font-semibold transition', tab === item ? 'bg-muted text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground')}>{item === 'overview' ? 'Overview' : 'Details'}</button>)}
        </div>
        <div className="flex items-center justify-between gap-2 sm:justify-end">
          <div className="inline-flex rounded-xl border border-border/80 bg-card/90 p-1 shadow-sm">
            {RANGES.map(item => <button key={item.value} onClick={() => setRange(item.value)} className={cn('rounded-lg px-2.5 py-2 text-xs font-semibold transition', range === item.value ? 'bg-muted text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground')}>{item.label}</button>)}
          </div>
          <Button variant="outline" size="icon" className="size-9" onClick={fetchData} aria-label="Refresh dashboard"><RefreshCw className="size-3.5" /></Button>
        </div>
      </header>

      {tab === 'overview' ? <>
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          {loading ? Array.from({ length: 4 }).map((_, index) => <Skeleton key={index} className="h-24 rounded-xl" />) : <>
            <StatCard label="Total requests" value={(summary?.totalRequests ?? 0).toLocaleString()} caption={`${range === '24h' ? 'Today' : `Last ${range}`}`} />
            <StatCard label="Total input tokens" value={formatTokens(summary?.inputTokens ?? 0)} accent="text-orange-400" />
            <StatCard label="Output tokens" value={formatTokens(summary?.outputTokens ?? 0)} accent="text-emerald-400" />
            <StatCard label="Est. cost" value={`~${formatCost(summary?.costTotal ?? 0)}`} caption="Estimated, not actual billing" accent="text-amber-400" />
          </>}
        </div>
        <div className="grid gap-3 lg:grid-cols-[minmax(0,1.9fr)_minmax(320px,0.95fr)]">
          <RoutingTopology connections={connections} summaries={connectionSummaries} combos={combos} aliases={aliases} requests={requests} onNavigate={navigate} />
          <RecentRequests requests={requests} loading={loading} onViewAll={() => navigate('/logs')} />
        </div>
      </> : <Details summary={summary} connections={connections} requests={requests} loading={loading} lastRefreshed={lastRefreshed} onPlayground={() => navigate('/playground')} />}
    </div>
  </div>
}

function Details({ summary, connections, requests, loading, lastRefreshed, onPlayground }: { summary: Summary | null; connections: Connection[]; requests: LogEntry[]; loading: boolean; lastRefreshed: number; onPlayground: () => void }) {
  const failures = requests.filter(entry => entry.level === 'ERROR' || (entry.statusCode ?? 0) >= 400).length
  return <section className="grid gap-3 lg:grid-cols-3">
    <div className="rounded-xl border border-border/80 bg-card p-5 lg:col-span-2"><div className="flex items-center gap-2"><Activity className="size-4 text-blue-400" /><h2 className="text-sm font-semibold">Proxy status</h2></div><p className="mt-4 text-2xl font-bold text-emerald-400">{loading ? 'Updating…' : failures ? 'Attention needed' : 'Healthy'}</p><p className="mt-2 text-sm text-muted-foreground">{connections.filter(connection => connection.isActive).length} active connections · {summary?.errorRequests ?? 0} errors in the selected period · Updated {formatRelativeTime(new Date(lastRefreshed).toISOString())}</p></div>
    <div className="rounded-xl border border-border/80 bg-card p-5"><h2 className="text-sm font-semibold">Ready to test?</h2><p className="mt-2 text-sm text-muted-foreground">Send a request through your current routing configuration.</p><Button className="mt-4 w-full gap-1.5" onClick={onPlayground}><Zap className="size-3.5" /> Open Playground</Button></div>
  </section>
}
