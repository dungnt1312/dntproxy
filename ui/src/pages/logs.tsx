import { useCallback, useEffect, useRef, useState } from 'react'

import { api } from '../api'
import LogConnectionList from '../components/log-connection-list'
import LogFilterBar from '../components/log-filter-bar'
import LogSummaryBar from '../components/log-summary-bar'
import LogTable from '../components/log-table'
import type { LogConnectionSummary, LogEntry, LogFilters, LogSummary } from '../types/logs'

const defaultFilters: LogFilters = {
  range: '24h',
  connectionId: 'all',
  provider: 'all',
  level: 'all',
  q: '',
}

export default function Logs() {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [summary, setSummary] = useState<LogSummary | null>(null)
  const [connections, setConnections] = useState<LogConnectionSummary[]>([])
  const [filters, setFilters] = useState(defaultFilters)
  const [live, setLive] = useState(true)
  const eventSourceRef = useRef<EventSource | null>(null)

  const loadLogs = useCallback(async () => {
    const [logData, summaryData, connectionData] = await Promise.all([
      api.getLogs(filters),
      api.getLogSummary(filters),
      api.getLogConnections({ range: filters.range }),
    ])
    setLogs(Array.isArray(logData) ? logData : [])
    setSummary(summaryData)
    setConnections(Array.isArray(connectionData) ? connectionData : [])
  }, [filters])

  useEffect(() => {
    loadLogs().catch(console.error)
  }, [loadLogs])

  useEffect(() => {
    eventSourceRef.current?.close()
    if (!live) return

    const query = new URLSearchParams()
    Object.entries(filters).forEach(([key, value]) => {
      if (value && value !== 'all') query.set(key, value)
    })
    const es = new EventSource(`/api/logs/stream?${query.toString()}`)
    es.onmessage = e => {
      const streamLogs = JSON.parse(e.data)
      setLogs(Array.isArray(streamLogs) ? streamLogs : [])
      api.getLogSummary(filters).then(setSummary).catch(console.error)
      api.getLogConnections({ range: filters.range })
        .then(data => setConnections(Array.isArray(data) ? data : []))
        .catch(console.error)
    }
    es.onerror = () => es.close()
    eventSourceRef.current = es

    return () => es.close()
  }, [filters, live])

  async function clearLogs() {
    await api.clearLogs()
    setLogs([])
    setSummary(null)
    setConnections([])
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="page-header mb-0">
        <div>
          <h2 className="page-title flex items-center gap-2">Logs</h2>
          <p className="page-subtitle">Request timeline with payload and cost tracking.</p>
        </div>
        <span className="chip chip-muted text-[10px]">30-day retention</span>
      </div>

      <LogSummaryBar summary={summary} />
      <p className="text-[10px] text-[var(--text-dim)] -mt-2">
        Cost is estimated from local model price profiles; Kiro billing can differ from token API pricing.
      </p>
      <LogFilterBar
        filters={filters}
        live={live}
        onFiltersChange={setFilters}
        onLiveChange={setLive}
        onRefresh={loadLogs}
        onClear={clearLogs}
      />

      <div className="flex flex-col gap-4 lg:flex-row lg:items-start min-h-0">
        <LogConnectionList
          connections={connections}
          selectedId={filters.connectionId}
          onSelect={connectionId => setFilters({ ...filters, connectionId })}
        />
        <main className="flex-1 min-w-0 space-y-2">
          <div className="flex flex-col gap-1 border-b border-[var(--border)] pb-2">
            <h3 className="text-sm font-semibold" style={{ fontFamily: 'var(--font-heading)' }}>Request timeline</h3>
            <p className="text-[10px] text-[var(--text-dim)]">
              Payload previews are captured as separate response events and capped before being stored.
            </p>
          </div>
          <LogTable logs={logs} />
        </main>
      </div>
    </div>
  )
}
