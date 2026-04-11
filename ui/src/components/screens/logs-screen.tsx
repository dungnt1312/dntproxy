import { useCallback, useEffect, useState } from 'react'
import { RefreshCw, Trash2 } from 'lucide-react'
import { toast } from 'sonner'

import { goApi } from '@/lib/go-api'

type LogEntry = {
  id: string
  timestamp?: string
  level?: string
  provider?: string
  direction?: string
  method?: string
  path?: string
  statusCode?: number
  model?: string
  connectionName?: string
  message?: string
  error?: string
  durationMs?: number
}

function formatTime(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString([], {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

export default function LogsScreen() {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [range, setRange] = useState('24h')

  const fetchLogs = useCallback(async () => {
    setLoading(true)
    try {
      const data = await goApi.getLogs({ range, limit: 200 })
      setLogs(Array.isArray(data) ? data : [])
    } catch {
      toast.error('Failed to load logs')
    } finally {
      setLoading(false)
    }
  }, [range])

  useEffect(() => {
    fetchLogs()
  }, [fetchLogs])

  async function handleRefresh() {
    setRefreshing(true)
    try {
      const data = await goApi.getLogs({ range, limit: 200 })
      setLogs(Array.isArray(data) ? data : [])
    } catch {
      toast.error('Failed to refresh logs')
    } finally {
      setRefreshing(false)
    }
  }

  async function handleClear() {
    if (!window.confirm('Clear all logs? This cannot be undone.')) {
      return
    }

    try {
      await goApi.clearLogs()
      setLogs([])
      toast.success('Logs cleared')
    } catch {
      toast.error('Failed to clear logs')
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Logs</h1>
          <p className="text-sm text-muted-foreground">Recent request and provider events.</p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <select
            value={range}
            onChange={(e) => setRange(e.target.value)}
            className="h-9 min-w-[140px] rounded-md border bg-background px-2 text-sm"
          >
            <option value="1h">Last hour</option>
            <option value="24h">Last 24h</option>
            <option value="7d">Last 7 days</option>
            <option value="30d">Last 30 days</option>
          </select>

          <button
            onClick={handleRefresh}
            className="inline-flex h-9 items-center justify-center rounded-md border px-3 text-sm hover:bg-accent"
            disabled={refreshing}
          >
            <RefreshCw className={`mr-1 h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} />
            Refresh
          </button>

          <button
            onClick={handleClear}
            className="inline-flex h-9 items-center justify-center rounded-md border border-destructive/30 px-3 text-sm text-destructive hover:bg-destructive/10"
          >
            <Trash2 className="mr-1 h-4 w-4" />
            Clear
          </button>
        </div>
      </div>

      <div className="rounded-lg border">
        {loading ? (
          <div className="p-6 text-sm text-muted-foreground">Loading logs...</div>
        ) : logs.length === 0 ? (
          <div className="p-8 text-center text-sm text-muted-foreground">No logs found for selected range.</div>
        ) : (
          <div className="max-h-[68vh] overflow-auto">
            <div className="hidden min-w-[860px] sm:block">
              <table className="w-full border-collapse text-sm">
                <thead className="sticky top-0 bg-background">
                  <tr className="border-b">
                    <th className="px-3 py-2 text-left font-medium">Time</th>
                    <th className="px-3 py-2 text-left font-medium">Provider</th>
                    <th className="px-3 py-2 text-left font-medium">Direction</th>
                    <th className="px-3 py-2 text-left font-medium">Status</th>
                    <th className="px-3 py-2 text-left font-medium">Path</th>
                    <th className="px-3 py-2 text-left font-medium">Message</th>
                  </tr>
                </thead>
                <tbody>
                  {logs.map((log) => {
                    const status = log.statusCode || (log.level === 'ERROR' ? 500 : 200)
                    return (
                      <tr key={log.id} className="border-b align-top hover:bg-muted/30">
                        <td className="px-3 py-2 text-xs text-muted-foreground whitespace-nowrap">{formatTime(log.timestamp)}</td>
                        <td className="px-3 py-2 whitespace-nowrap">{log.provider || '-'}</td>
                        <td className="px-3 py-2 whitespace-nowrap">{log.direction || '-'}</td>
                        <td className="px-3 py-2 whitespace-nowrap">{status}</td>
                        <td className="px-3 py-2 font-mono text-xs">{log.path || '-'}</td>
                        <td className="px-3 py-2">
                          <div className="text-sm">{log.message || log.error || '-'}</div>
                          <div className="mt-1 text-xs text-muted-foreground">
                            {log.connectionName ? `${log.connectionName}` : ''}
                            {log.model ? `${log.connectionName ? ' · ' : ''}${log.model}` : ''}
                            {log.durationMs ? `${log.connectionName || log.model ? ' · ' : ''}${log.durationMs}ms` : ''}
                          </div>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>

            <div className="space-y-3 p-3 sm:hidden">
              {logs.map((log) => {
                const status = log.statusCode || (log.level === 'ERROR' ? 500 : 200)
                return (
                  <div key={log.id} className="rounded-lg border p-3 text-sm">
                    <div className="mb-2 flex items-start justify-between gap-2">
                      <span className="text-xs text-muted-foreground">{formatTime(log.timestamp)}</span>
                      <span className="rounded bg-muted px-2 py-0.5 text-xs font-medium">{status}</span>
                    </div>
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">
                        {log.provider || '-'} · {log.direction || '-'}
                      </p>
                      <p className="break-all font-mono text-xs">{log.path || '-'}</p>
                      <p>{log.message || log.error || '-'}</p>
                      <p className="text-xs text-muted-foreground">
                        {log.connectionName ? `${log.connectionName}` : ''}
                        {log.model ? `${log.connectionName ? ' · ' : ''}${log.model}` : ''}
                        {log.durationMs ? `${log.connectionName || log.model ? ' · ' : ''}${log.durationMs}ms` : ''}
                      </p>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
