import { useEffect, useState, useRef, useCallback } from 'react'
import { Radio, Pause, Play, X, ChevronDown, ChevronRight } from 'lucide-react'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { ProviderLogo } from '@/components/connections/ProviderLogo'
import { cn } from '@/lib/utils'
import { goApi } from '@/lib/go-api'
import type { LogEntry } from '@/types/logs'

const POLL_INTERVAL_MS = 3000
const MAX_ENTRIES = 30

export function LiveRequestStream() {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [paused, setPaused] = useState(false)
  const [open, setOpen] = useState(true)
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

  const handleClear = () => setEntries([])

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <Card>
        <CardHeader className="pb-2">
          <div className="flex items-center justify-between">
            <CollapsibleTrigger asChild>
              <Button variant="ghost" size="sm" className="gap-1.5 px-2">
                {open ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                <Radio className={cn('size-3.5', !paused && entries.length > 0 && 'text-emerald-500 animate-pulse')} />
                <span className="text-sm font-semibold">Live Request Stream</span>
                {entries.length > 0 && (
                  <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
                    {entries.length}
                  </Badge>
                )}
              </Button>
            </CollapsibleTrigger>

            <div className="flex gap-1">
              <Button
                variant="ghost"
                size="icon"
                className="size-7"
                onClick={() => setPaused(p => !p)}
              >
                {paused ? <Play className="size-3" /> : <Pause className="size-3" />}
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="size-7"
                onClick={handleClear}
              >
                <X className="size-3" />
              </Button>
            </div>
          </div>
        </CardHeader>
        <CollapsibleContent>
          <CardContent>
            {entries.length === 0 ? (
              <div className="text-center py-6 text-muted-foreground text-sm">
                {paused ? 'Polling paused' : 'Waiting for requests...'}
              </div>
            ) : (
              <ScrollArea className="h-[280px]">
                <div className="space-y-1">
                  {entries.map(entry => (
                    <RequestEntry key={entry.id} entry={entry} />
                  ))}
                </div>
              </ScrollArea>
            )}
          </CardContent>
        </CollapsibleContent>
      </Card>
    </Collapsible>
  )
}

function RequestEntry({ entry }: { entry: LogEntry }) {
  const isError = entry.level === 'ERROR' || (entry.statusCode && entry.statusCode >= 400)
  const time = entry.timestamp ? new Date(entry.timestamp).toLocaleTimeString('en-US', { hour12: false }) : ''

  return (
    <div
      className={cn(
        'flex items-center gap-2 px-2 py-1.5 rounded-md text-xs transition-colors',
        isError ? 'bg-red-50/50 dark:bg-red-950/10' : 'hover:bg-muted/50',
      )}
    >
      <span className="text-muted-foreground font-mono shrink-0 w-16">{time}</span>
      <span className="font-mono truncate max-w-[200px]" title={entry.model}>
        {entry.model || entry.path || '-'}
      </span>
      <span className="text-muted-foreground">→</span>
      <div className="flex items-center gap-1 shrink-0">
        {entry.provider && <ProviderLogo provider={entry.provider} size={14} />}
        <span className="max-w-[120px] truncate">{entry.connectionName || entry.provider || '-'}</span>
      </div>
      {isError ? (
        <Badge variant="destructive" className="text-[9px] px-1 py-0 shrink-0">
          {entry.statusCode || 'ERR'}
        </Badge>
      ) : (
        <Badge variant="outline" className="text-[9px] px-1 py-0 shrink-0 text-emerald-600 border-emerald-200 dark:text-emerald-400 dark:border-emerald-800">
          {entry.statusCode || 'OK'}
        </Badge>
      )}
      {entry.durationMs != null && entry.durationMs > 0 && (
        <span className={cn(
          'shrink-0',
          entry.durationMs < 1000 ? 'text-emerald-600 dark:text-emerald-400' :
          entry.durationMs < 3000 ? 'text-amber-600 dark:text-amber-400' :
          'text-red-600 dark:text-red-400',
        )}>
          {entry.durationMs < 1000 ? `${entry.durationMs}ms` : `${(entry.durationMs / 1000).toFixed(1)}s`}
        </span>
      )}
      {entry.totalTokens != null && entry.totalTokens > 0 && (
        <span className="text-muted-foreground shrink-0">{formatTok(entry.totalTokens)} tok</span>
      )}
    </div>
  )
}

function formatTok(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`
  return String(n)
}
