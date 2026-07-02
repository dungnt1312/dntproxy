import { useEffect, useState, useCallback, useRef } from 'react'
import { Radio, Pause, Play, X } from 'lucide-react'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { ProviderLogo } from '@/components/connections/ProviderLogo'
import { cn } from '@/lib/utils'
import { goApi } from '@/lib/go-api'
import { formatTokens } from './helpers'
import type { LogEntry } from '@/types/logs'

// ─── Live Feed (API poll) ────────────────────────────────────────────────────

const POLL_INTERVAL_MS = 3000
const MAX_ENTRIES = 50

export function LiveFeed() {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [paused, setPaused] = useState(false)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const fetchLogs = useCallback(async () => {
    try {
      const res = await goApi.getLogs({ range: '1h', limit: MAX_ENTRIES })
      if (Array.isArray(res)) {
        setEntries(res.slice(0, MAX_ENTRIES))
      }
    } catch {
      // retry on next poll
    }
  }, [])

  useEffect(() => {
    if (paused) {
      if (pollRef.current) {
        clearInterval(pollRef.current)
        pollRef.current = null
      }
      return
    }

    fetchLogs()
    pollRef.current = setInterval(fetchLogs, POLL_INTERVAL_MS)

    return () => {
      if (pollRef.current) {
        clearInterval(pollRef.current)
        pollRef.current = null
      }
    }
  }, [paused, fetchLogs])

  return (
    <Card className="flex flex-col overflow-hidden">
      <CardHeader className="pb-2 shrink-0">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold flex items-center gap-1.5">
            <Radio className={cn('size-3.5', !paused && entries.length > 0 && 'text-emerald-500 animate-pulse')} />
            Live Requests
            {entries.length > 0 && (
              <Badge variant="secondary" className="text-[9px] px-1.5 py-0">
                {entries.length}
              </Badge>
            )}
          </h2>
          <div className="flex gap-1">
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              title={paused ? 'Resume' : 'Pause'}
              onClick={() => setPaused(p => !p)}
            >
              {paused ? <Play className="size-3" /> : <Pause className="size-3" />}
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              title="Clear"
              onClick={() => setEntries([])}
            >
              <X className="size-3" />
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="pt-0 flex-1 overflow-hidden">
        {entries.length === 0 ? (
          <div className="flex items-center justify-center h-full text-muted-foreground text-sm py-10">
            {paused ? 'Polling paused' : 'Waiting for requests...'}
          </div>
        ) : (
          <ScrollArea className="flex-1 min-h-0">
            <table className="w-full text-xs border-collapse">
              <thead className="sticky top-0 bg-card z-10">
                <tr className="border-b border-border">
                  <th className="py-1.5 w-2"></th>
                  <th className="py-1.5 text-left font-semibold text-muted-foreground text-[10px] uppercase tracking-wider">
                    Time
                  </th>
                  <th className="py-1.5 text-left font-semibold text-muted-foreground text-[10px] uppercase tracking-wider">
                    Model
                  </th>
                  <th className="py-1.5 text-left font-semibold text-muted-foreground text-[10px] uppercase tracking-wider">
                    Connection
                  </th>
                  <th className="py-1.5 text-center font-semibold text-muted-foreground text-[10px] uppercase tracking-wider">
                    Status
                  </th>
                  <th className="py-1.5 text-right font-semibold text-muted-foreground text-[10px] uppercase tracking-wider whitespace-nowrap">
                    Tokens
                  </th>
                  <th className="py-1.5 text-right font-semibold text-muted-foreground text-[10px] uppercase tracking-wider">
                    Duration
                  </th>
                  <th className="py-1.5 text-right font-semibold text-muted-foreground text-[10px] uppercase tracking-wider">
                    Cost
                  </th>
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
  
  // Format cost
  const cost = entry.costTotal || 0
  const costStr = cost > 0 ? `$${cost.toFixed(4)}` : '—'
  
  // Format time
  const time = new Date(entry.timestamp).toLocaleTimeString('en-US', { 
    hour: '2-digit', 
    minute: '2-digit',
    second: '2-digit',
    hour12: false 
  })

  return (
    <tr className={cn('transition-colors', isError ? 'bg-red-500/10 dark:bg-red-500/15' : 'hover:bg-muted/30')}>
      <td className="py-1.5 pl-1">
        <span className={cn('block w-1.5 h-1.5 rounded-full', isError ? 'bg-red-500' : 'bg-emerald-500')} />
      </td>
      <td className="py-1.5 text-muted-foreground font-mono text-[10px] whitespace-nowrap">
        {time}
      </td>
      <td className="py-1.5 font-mono truncate max-w-[120px]" title={entry.model}>
        {entry.model?.split('/').pop() || entry.path || '—'}
      </td>
      <td className="py-1.5">
        <div className="flex items-center gap-1">
          {entry.provider && <ProviderLogo provider={entry.provider} size={12} />}
          <span className="truncate max-w-[80px] text-muted-foreground text-[10px]">
            {entry.connectionName || '—'}
          </span>
        </div>
      </td>
      <td className="py-1.5 text-center">
        <span className={cn(
          'inline-block px-1.5 py-0.5 rounded text-[9px] font-medium',
          isError 
            ? 'bg-red-500/20 text-red-600 dark:text-red-400' 
            : 'bg-emerald-500/20 text-emerald-600 dark:text-emerald-400'
        )}>
          {entry.statusCode || (isError ? 'ERR' : 'OK')}
        </span>
      </td>
      <td className="py-1.5 text-right whitespace-nowrap tabular-nums">
        {(entry.totalTokens || 0) > 0
          ? <span className="text-emerald-400">{formatTokens(entry.totalTokens || 0)}</span>
          : <span className="text-muted-foreground/40">—</span>}
      </td>
      <td className="py-1.5 text-right text-muted-foreground whitespace-nowrap font-mono">
        {entry.durationMs
          ? (entry.durationMs < 1000 ? `${entry.durationMs}ms` : `${(entry.durationMs / 1000).toFixed(1)}s`)
          : <span className="text-muted-foreground/40">—</span>}
      </td>
      <td className="py-1.5 text-right whitespace-nowrap tabular-nums">
        <span className={cn(cost > 0 ? 'text-amber-500' : 'text-muted-foreground/40')}>
          {costStr}
        </span>
      </td>
    </tr>
  )
}
