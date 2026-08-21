import { useMemo, useState } from 'react'
import { Box, Crosshair, GitBranch, Minus, Plus, Route, Unplug } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ProviderLogo } from '@/components/connections/ProviderLogo'
import { cn } from '@/lib/utils'
import type { Connection } from '@/types/connections'
import type { LogConnectionSummary, LogEntry } from '@/types/logs'
import type { AliasMap, ComboData } from '@/components/screens/routing/types'

interface RoutingTopologyProps {
  connections: Connection[]
  summaries: LogConnectionSummary[]
  combos: ComboData[]
  aliases: AliasMap
  requests: LogEntry[]
  onNavigate: (path: string) => void
}

type NodeStatus = 'active' | 'standby' | 'attention'

function nodeStatus(connection: Connection, summary?: LogConnectionSummary): NodeStatus {
  if (connection.lastError || (connection.rateLimitedUntil && new Date(connection.rateLimitedUntil).getTime() > Date.now())) return 'attention'
  if ((summary?.lastUsedMs ?? 0) > Date.now() - 5 * 60_000) return 'active'
  return 'standby'
}

function modelLabel(model: string) {
  return model.replace(/^\w+\//, '').replace(/@.+$/, '') || model
}

export function RoutingTopology({ connections, summaries, combos, aliases, requests, onNavigate }: RoutingTopologyProps) {
  const [scale, setScale] = useState(1)
  const summaryByConnection = useMemo(() => new Map(summaries.map(summary => [summary.connectionId, summary])), [summaries])
  const recentRequest = requests.find(request => request.model && request.connectionId)
  const recentModel = recentRequest?.model || ''
  const recentProvider = recentRequest?.provider || ''
  const primaryCombo = combos[0]
  const aliasEntries = Object.entries(aliases).slice(0, 2)
  const targetConnections = useMemo(() => {
    if (recentRequest) {
      const observed = connections.filter(connection => connection.provider === recentProvider || connection.id === recentRequest.connectionId)
      return (observed.length ? observed : connections.filter(connection => connection.id === recentRequest.connectionId)).slice(0, 4)
    }
    if (!primaryCombo) return connections.slice(0, 4)
    const pinnedIds = primaryCombo.models
      .map(model => model.split('@')[1])
      .filter((id): id is string => Boolean(id && id !== 'auto'))
    const compatible = connections.filter(connection =>
      pinnedIds.includes(connection.id) || primaryCombo.models.some(model => model.startsWith(`${connection.routePrefix || connection.provider}/`)),
    )
    return (compatible.length ? compatible : connections).slice(0, 4)
  }, [connections, primaryCombo, recentProvider, recentRequest])

  const sourceLabel = recentModel ? modelLabel(recentModel) : primaryCombo?.models[0] ? modelLabel(primaryCombo.models[0]) : aliasEntries[0]?.[0] || 'Incoming requests'
  const routeLabel = recentModel ? (recentProvider || 'Observed route') : primaryCombo?.name || 'Router'
  const routeDetail = recentModel ? 'recent traffic' : primaryCombo ? `${primaryCombo.models.length} route${primaryCombo.models.length === 1 ? '' : 's'}` : `${aliasEntries.length} alias${aliasEntries.length === 1 ? '' : 'es'}`
  const activeConnection = targetConnections.find(connection => nodeStatus(connection, summaryByConnection.get(connection.id)) === 'active')
  const hasConfig = primaryCombo || aliasEntries.length > 0 || targetConnections.length > 0

  return (
    <section className="overflow-hidden rounded-xl border border-border/80 bg-card shadow-sm">
      <div className="flex items-center justify-between border-b border-border/70 px-4 py-3">
        <div>
          <div className="flex items-center gap-2 text-sm font-semibold"><Route className="size-4 text-blue-400" /> Routing topology</div>
          <p className="mt-0.5 text-[11px] text-muted-foreground">Configuration with recent observed traffic</p>
        </div>
        <Button variant="ghost" size="sm" className="h-7 gap-1.5 text-xs" onClick={() => onNavigate('/combos')}>
          <GitBranch className="size-3.5" /> Manage
        </Button>
      </div>

      {!hasConfig ? (
        <div className="flex h-[390px] flex-col items-center justify-center gap-3 px-6 text-center">
          <Unplug className="size-8 text-muted-foreground" />
          <div><p className="text-sm font-medium">No routing configured</p><p className="mt-1 text-xs text-muted-foreground">Add a connection or combo to see your route map.</p></div>
          <Button size="sm" onClick={() => onNavigate('/connections')}>Add connection</Button>
        </div>
      ) : (
        <div className="relative h-[390px] overflow-hidden dashboard-grid">
          <div className="absolute inset-0 origin-center transition-transform duration-200" style={{ transform: `scale(${scale})` }}>
            <svg className="pointer-events-none absolute inset-0 h-full w-full" viewBox="0 0 800 390" preserveAspectRatio="none" aria-hidden="true">
              <path d="M400 93 C400 128 400 144 400 176" className="fill-none stroke-blue-400/80" strokeWidth="2" strokeDasharray="6 5" />
              {targetConnections.map((connection, index) => {
                const x = targetConnections.length === 1 ? 400 : 145 + index * (510 / Math.max(targetConnections.length - 1, 1))
                const status = nodeStatus(connection, summaryByConnection.get(connection.id))
                return <path key={connection.id} d={`M400 222 C400 265 ${x} 265 ${x} 300`} className={cn('fill-none', status === 'active' ? 'stroke-emerald-400/70' : status === 'attention' ? 'stroke-orange-400/60' : 'stroke-muted-foreground/30')} strokeWidth="1.5" strokeDasharray={status === 'active' ? '5 5' : undefined} />
              })}
            </svg>

            <div className="absolute left-1/2 top-10 -translate-x-1/2">
              <GraphNode label={sourceLabel} detail={recentModel ? 'observed model' : primaryCombo ? 'entry model' : 'alias route'} status="active" onClick={() => onNavigate(recentModel ? '/logs' : primaryCombo ? '/combos' : '/models')} />
            </div>
            <div className="absolute left-1/2 top-[166px] -translate-x-1/2">
              <GraphNode label={routeLabel} detail={routeDetail} router status={activeConnection || recentModel ? 'active' : 'standby'} onClick={() => onNavigate(recentModel ? '/logs' : primaryCombo ? '/combos' : '/models')} />
            </div>
            {targetConnections.map((connection, index) => {
              const left = targetConnections.length === 1 ? 50 : 18 + index * (64 / Math.max(targetConnections.length - 1, 1))
              const summary = summaryByConnection.get(connection.id)
              return <div key={connection.id} className="absolute top-[298px] -translate-x-1/2" style={{ left: `${left}%` }}>
                <ConnectionNode connection={connection} summary={summary} status={nodeStatus(connection, summary)} onClick={() => onNavigate(`/logs?connectionId=${connection.id}`)} />
              </div>
            })}
          </div>
          <div className="absolute bottom-3 left-3 flex overflow-hidden rounded-md border border-border bg-background/90 shadow-sm">
            <Button variant="ghost" size="icon" className="size-7 rounded-none" onClick={() => setScale(value => Math.min(1.18, value + 0.09))}><Plus className="size-3.5" /></Button>
            <Button variant="ghost" size="icon" className="size-7 rounded-none border-x border-border" onClick={() => setScale(value => Math.max(0.82, value - 0.09))}><Minus className="size-3.5" /></Button>
            <Button variant="ghost" size="icon" className="size-7 rounded-none" onClick={() => setScale(1)}><Crosshair className="size-3.5" /></Button>
          </div>
        </div>
      )}
    </section>
  )
}

function GraphNode({ label, detail, status, router, onClick }: { label: string; detail: string; status: NodeStatus; router?: boolean; onClick: () => void }) {
  return <button onClick={onClick} className={cn('flex min-w-36 items-center gap-2 rounded-lg border bg-card/95 px-3 py-2 text-left shadow-lg transition hover:-translate-y-0.5 hover:border-blue-400/70', router ? 'border-orange-400/70' : status === 'active' ? 'border-blue-400/80' : status === 'attention' ? 'border-orange-400/70' : 'border-border')}>
    <span className={cn('flex size-6 items-center justify-center rounded-md', router ? 'bg-orange-500/15 text-orange-400' : 'bg-blue-500/15 text-blue-400')}>{router ? <GitBranch className="size-3.5" /> : <Box className="size-3.5" />}</span>
    <span className="min-w-0"><span className="block max-w-32 truncate text-xs font-semibold">{label}</span><span className="block text-[10px] text-muted-foreground">{detail} <i className={cn('ml-1 inline-block size-1.5 rounded-full', status === 'active' ? 'bg-emerald-400' : status === 'attention' ? 'bg-orange-400' : 'bg-muted-foreground')} /></span></span>
  </button>
}

function ConnectionNode({ connection, summary, status, onClick }: { connection: Connection; summary?: LogConnectionSummary; status: NodeStatus; onClick: () => void }) {
  const detail = status === 'active' ? 'active now' : status === 'attention' ? (connection.lastError ? 'needs attention' : 'rate limited') : summary?.lastUsedMs ? 'recent traffic' : 'standby'
  return <button onClick={onClick} className={cn('flex w-36 items-center gap-2 rounded-lg border bg-card/95 px-2.5 py-2 text-left shadow-md transition hover:-translate-y-0.5', status === 'active' ? 'border-emerald-400/60' : status === 'attention' ? 'border-orange-400/60' : 'border-border')}>
    <ProviderLogo provider={connection.provider} size={18} className="shrink-0" />
    <span className="min-w-0"><span className="block truncate text-[11px] font-semibold">{connection.name}</span><span className="block truncate text-[10px] text-muted-foreground">{detail}</span></span>
  </button>
}
