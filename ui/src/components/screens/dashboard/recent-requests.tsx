import { ArrowRight, Circle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { formatRelativeTime, formatTokens } from './helpers'
import type { LogEntry } from '@/types/logs'

interface RecentRequestsProps {
  requests: LogEntry[]
  loading: boolean
  onViewAll: () => void
}

export function RecentRequests({ requests, loading, onViewAll }: RecentRequestsProps) {
  return (
    <section className="flex h-full min-h-[390px] flex-col overflow-hidden rounded-xl border border-border/80 bg-card shadow-sm">
      <div className="flex items-center justify-between border-b border-border/70 px-4 py-3">
        <div><h2 className="text-sm font-semibold">Recent requests</h2><p className="mt-0.5 text-[11px] text-muted-foreground">Live proxy activity</p></div>
        <span className="flex items-center gap-1.5 text-[10px] font-medium text-emerald-400"><span className="size-1.5 animate-pulse rounded-full bg-emerald-400" /> LIVE</span>
      </div>
      <div className="grid grid-cols-[minmax(0,1fr)_72px_46px] gap-2 border-b border-border/70 bg-muted/20 px-4 py-2 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
        <span>Model</span><span className="text-right">In / Out</span><span className="text-right">When</span>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-2 py-1">
        {loading ? <div className="space-y-2 p-2">{Array.from({ length: 8 }).map((_, index) => <div key={index} className="h-7 animate-pulse rounded bg-muted/60" />)}</div>
          : requests.length === 0 ? <div className="flex h-full items-center justify-center p-6 text-center text-xs text-muted-foreground">Waiting for proxy requests…</div>
          : requests.map(request => <RequestRow key={request.id} request={request} />)}
      </div>
      <div className="border-t border-border/70 p-2"><Button variant="ghost" size="sm" className="h-8 w-full gap-1.5 text-xs text-muted-foreground hover:text-foreground" onClick={onViewAll}>View all logs <ArrowRight className="size-3" /></Button></div>
    </section>
  )
}

function RequestRow({ request }: { request: LogEntry }) {
  const failed = request.level === 'ERROR' || (request.statusCode ?? 0) >= 400
  const input = formatTokens(request.inputTokens ?? 0)
  const output = formatTokens(request.outputTokens ?? 0)
  return <button className="grid w-full grid-cols-[minmax(0,1fr)_72px_46px] items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition hover:bg-muted/60" title={request.error || request.message}>
    <span className="flex min-w-0 items-center gap-1.5"><Circle className={cn('size-1.5 shrink-0 fill-current', failed ? 'text-orange-400' : 'text-emerald-400')} /><span className="truncate font-mono text-[11px]">{request.model || request.path || 'Unknown model'}</span></span>
    <span className={cn('text-right font-mono text-[10px]', failed ? 'text-orange-400' : 'text-muted-foreground')}><span className="text-orange-400">{input}</span> <span className="text-emerald-400">{output}</span></span>
    <span className="text-right text-[10px] text-muted-foreground">{formatRelativeTime(request.timestamp)}</span>
  </button>
}
