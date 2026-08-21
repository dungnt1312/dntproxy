import { useMemo, useState } from 'react'
import { Crosshair, Minus, Network, Plus, Route } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ProviderLogo } from '@/components/connections/ProviderLogo'
import { cn } from '@/lib/utils'
import { canonicalProviderId, getProviderLabel, providersMatch } from '@/lib/provider-registry'
import type { Connection } from '@/types/connections'
import type { LogConnectionSummary, LogEntry } from '@/types/logs'

interface RoutingTopologyProps {
  connections: Connection[]
  summaries: LogConnectionSummary[]
  requests: LogEntry[]
  onNavigate: (path: string) => void
}

type NodeStatus = 'active' | 'standby' | 'attention'

interface ProviderHub {
  id: string
  label: string
  connections: Connection[]
  requests: number
  tokens: number
  lastUsedMs: number
  activeConnections: number
  attentionConnections: number
  status: NodeStatus
}

function connectionNeedsAttention(connection: Connection) {
  return Boolean(connection.lastError || (connection.rateLimitedUntil && new Date(connection.rateLimitedUntil).getTime() > Date.now()))
}

function providerStatus(connections: Connection[], lastUsedMs: number): NodeStatus {
  if (connections.some(connectionNeedsAttention)) return 'attention'
  return lastUsedMs > Date.now() - 5 * 60_000 ? 'active' : 'standby'
}

function buildProviderHubs(connections: Connection[], summaries: LogConnectionSummary[], requests: LogEntry[]) {
  const byProvider = new Map<string, Connection[]>()
  for (const connection of connections) {
    const id = canonicalProviderId(connection.provider)
    byProvider.set(id, [...(byProvider.get(id) ?? []), connection])
  }

  const hubs: ProviderHub[] = [...byProvider.entries()].map(([id, providerConnections]) => {
    const providerSummaries = summaries.filter(summary => providersMatch(summary.provider, id))
    const providerRequests = requests.filter(request => providersMatch(request.provider, id))
    const requestCount = providerSummaries.reduce((total, summary) => total + summary.requests, 0)
    const tokens = providerSummaries.reduce((total, summary) => total + summary.totalTokens, 0)
    const lastUsedMs = Math.max(0, ...providerSummaries.map(summary => summary.lastUsedMs), ...providerRequests.map(request => request.timestampMs || new Date(request.timestamp).getTime()))
    const activeConnections = providerConnections.filter(connection => {
      const summary = providerSummaries.find(item => item.connectionId === connection.id)
      return (summary?.lastUsedMs ?? 0) > Date.now() - 5 * 60_000
    }).length
    return { id, label: getProviderLabel(id), connections: providerConnections, requests: requestCount, tokens, lastUsedMs, activeConnections, attentionConnections: providerConnections.filter(connectionNeedsAttention).length, status: providerStatus(providerConnections, lastUsedMs) }
  })

  return hubs.sort((a, b) => {
    const rank = (hub: ProviderHub) => hub.status === 'attention' ? 0 : hub.status === 'active' ? 1 : 2
    return rank(a) - rank(b) || b.requests - a.requests || b.connections.length - a.connections.length
  })
}

export function RoutingTopology({ connections, summaries, requests, onNavigate }: RoutingTopologyProps) {
  const [scale, setScale] = useState(1)
  const [selectedProvider, setSelectedProvider] = useState<string | null>(null)
  const hubs = useMemo(() => buildProviderHubs(connections, summaries, requests), [connections, summaries, requests])
  const visibleHubs = hubs.slice(0, 6)
  const hiddenCount = hubs.length - visibleHubs.length
  const selectedHub = hubs.find(hub => hub.id === selectedProvider)
  const activeProviders = hubs.filter(hub => hub.lastUsedMs > Date.now() - 5 * 60_000).length

  return <section className="overflow-hidden rounded-xl border border-border/80 bg-card shadow-sm">
    <div className="flex items-center justify-between border-b border-border/70 px-4 py-3">
      <div><div className="flex items-center gap-2 text-sm font-semibold"><Route className="size-4 text-blue-400" /> Routing topology</div><p className="mt-0.5 text-[11px] text-muted-foreground">Live provider routing · select a provider to inspect connections</p></div>
      <Button variant="ghost" size="sm" className="h-7 gap-1.5 text-xs" onClick={() => onNavigate('/connections')}>Manage</Button>
    </div>
    {hubs.length === 0 ? <div className="flex h-[390px] items-center justify-center text-sm text-muted-foreground">Add a connection to see provider routing.</div> : <>
      <div className="relative h-[390px] overflow-hidden dashboard-grid">
        <div className="absolute inset-0 origin-center transition-transform duration-200" style={{ transform: `scale(${scale})` }}>
          <svg className="pointer-events-none absolute inset-0 h-full w-full" viewBox="0 0 800 390" preserveAspectRatio="none" aria-hidden="true">
            {visibleHubs.map((hub, index) => {
              if (hub.lastUsedMs <= Date.now() - 5 * 60_000) return null
              const position = hubPosition(index, visibleHubs.length)
              return <path key={hub.id} d={`M400 195 C${400 + (position.x - 400) * 0.42} ${195 + (position.y - 195) * 0.35}, ${400 + (position.x - 400) * 0.72} ${195 + (position.y - 195) * 0.72}, ${position.x} ${position.y}`} className="dashboard-flow fill-none stroke-emerald-400/75" strokeWidth={2} strokeDasharray="6 5" />
            })}
          </svg>
          <div className="absolute left-1/2 top-1/2 z-10 -translate-x-1/2 -translate-y-1/2"><DntproxyNode providers={hubs.length} connections={connections.length} active={activeProviders} onClick={() => onNavigate('/logs')} /></div>
          {visibleHubs.map((hub, index) => { const position = hubPosition(index, visibleHubs.length); return <div key={hub.id} className="absolute z-10 -translate-x-1/2 -translate-y-1/2" style={{ left: `${(position.x / 800) * 100}%`, top: `${(position.y / 390) * 100}%` }}><ProviderNode hub={hub} selected={selectedProvider === hub.id} onClick={() => setSelectedProvider(hub.id)} /></div> })}
          {hiddenCount > 0 && <button onClick={() => onNavigate('/connections')} className="absolute bottom-8 right-8 z-10 rounded-lg border border-border bg-card px-3 py-2 text-xs font-semibold text-muted-foreground shadow hover:text-foreground">+{hiddenCount} providers</button>}
        </div>
        <div className="absolute bottom-3 left-3 flex overflow-hidden rounded-md border border-border bg-background/90 shadow-sm"><Button variant="ghost" size="icon" className="size-7 rounded-none" onClick={() => setScale(value => Math.min(1.18, value + 0.09))}><Plus className="size-3.5" /></Button><Button variant="ghost" size="icon" className="size-7 rounded-none border-x border-border" onClick={() => setScale(value => Math.max(0.82, value - 0.09))}><Minus className="size-3.5" /></Button><Button variant="ghost" size="icon" className="size-7 rounded-none" onClick={() => setScale(1)}><Crosshair className="size-3.5" /></Button></div>
      </div>
      {selectedHub && <ProviderInspector hub={selectedHub} onClose={() => setSelectedProvider(null)} onNavigate={onNavigate} />}
    </>}
  </section>
}

function hubPosition(index: number, total: number) {
  const positions = [{ x: 400, y: 62 }, { x: 648, y: 115 }, { x: 648, y: 292 }, { x: 400, y: 334 }, { x: 152, y: 292 }, { x: 152, y: 115 }]
  return positions[index] ?? positions[index % positions.length]
}

function DntproxyNode({ providers, connections, active, onClick }: { providers: number; connections: number; active: number; onClick: () => void }) {
  return <button onClick={onClick} className="flex min-w-40 items-center gap-2.5 rounded-xl border border-blue-400/80 bg-card px-4 py-3 text-left shadow-lg shadow-blue-500/10 transition hover:border-blue-300"><span className="flex size-8 items-center justify-center rounded-lg bg-blue-500/15 text-blue-400"><Network className="size-4" /></span><span><span className="block text-sm font-bold">dntproxy</span><span className="block text-[10px] text-muted-foreground">{providers} providers · {connections} connections</span><span className="block text-[10px] text-emerald-400">{active} active provider{active === 1 ? '' : 's'}</span></span></button>
}

function ProviderNode({ hub, selected, onClick }: { hub: ProviderHub; selected: boolean; onClick: () => void }) {
  const detail = hub.status === 'attention' ? `${hub.attentionConnections} needs attention` : hub.status === 'active' ? `${hub.activeConnections || 1} active · ${hub.requests} req` : `${hub.connections.length} connection${hub.connections.length === 1 ? '' : 's'} · standby`
  return <button onClick={onClick} className={cn('flex w-40 items-center gap-2 rounded-lg border bg-card/95 px-3 py-2.5 text-left shadow-md transition hover:-translate-y-0.5', selected ? 'border-blue-400 ring-1 ring-blue-400/30' : hub.status === 'active' ? 'border-emerald-400/60' : hub.status === 'attention' ? 'border-orange-400/70' : 'border-border')}><ProviderLogo provider={hub.id} size={20} className="shrink-0" /><span className="min-w-0"><span className="flex items-center gap-1"><span className="truncate text-xs font-semibold">{hub.label}</span><i className={cn('size-1.5 shrink-0 rounded-full', hub.status === 'active' ? 'bg-emerald-400' : hub.status === 'attention' ? 'bg-orange-400' : 'bg-muted-foreground')} /></span><span className="block truncate text-[10px] text-muted-foreground">{detail}</span></span></button>
}

function ProviderInspector({ hub, onClose, onNavigate }: { hub: ProviderHub; onClose: () => void; onNavigate: (path: string) => void }) {
  return <div className="border-t border-border/70 bg-muted/15 px-4 py-3"><div className="flex items-start justify-between gap-3"><div><div className="flex items-center gap-2 text-sm font-semibold"><ProviderLogo provider={hub.id} size={16} />{hub.label}</div><p className="mt-1 text-[11px] text-muted-foreground">{hub.requests} requests · {hub.connections.length} configured connections</p></div><div className="flex gap-1"><Button variant="ghost" size="sm" className="h-7 text-xs" onClick={() => onNavigate(`/logs?provider=${encodeURIComponent(hub.id)}`)}>View logs</Button><Button variant="ghost" size="sm" className="h-7 text-xs" onClick={onClose}>Close</Button></div></div><div className="mt-3 flex flex-wrap gap-2">{hub.connections.map(connection => <button key={connection.id} onClick={() => onNavigate(`/logs?connectionId=${encodeURIComponent(connection.id)}`)} className={cn('rounded-md border px-2.5 py-1.5 text-left text-xs transition hover:bg-muted', connectionNeedsAttention(connection) ? 'border-orange-400/50' : 'border-border')}><span className="block font-medium">{connection.name}</span><span className="block text-[10px] text-muted-foreground">{connectionNeedsAttention(connection) ? 'needs attention' : connection.isActive ? 'enabled' : 'disabled'}</span></button>)}</div></div>
}
