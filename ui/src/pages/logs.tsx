import { useEffect, useState, useRef } from 'react'
import { Terminal, Trash2, RefreshCw } from 'lucide-react'

interface LogEntry {
  timestamp: string
  level: string
  provider: string
  message: string
}

function parseLogMessage(msg: string) {
  const parts: Record<string, string> = {}
  
  const urlMatch = msg.match(/\b(https?:\/\/[^\s|]+)/)
  if (urlMatch) parts.url = urlMatch[1]
  
  const methodMatch = msg.match(/\b(GET|POST|PUT|DELETE)\b/)
  if (methodMatch) parts.method = methodMatch[1]
  
  const connMatch = msg.match(/conn=([^|\s]+)/)
  if (connMatch) parts.conn = connMatch[1]
  
  const modelMatch = msg.match(/model=([^|\s]+)/)
  if (modelMatch) parts.model = modelMatch[1]
  
  const statusMatch = msg.match(/status=(\d+)/)
  if (statusMatch) parts.status = statusMatch[1]
  
  const durationMatch = msg.match(/duration=([^|\s]+)/)
  if (durationMatch) parts.duration = durationMatch[1]
  
  const errorMatch = msg.match(/error=([^|\s]+)/)
  if (errorMatch) parts.error = errorMatch[1]
  
  return parts
}

export default function Logs() {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [autoRefresh, setAutoRefresh] = useState(true)
  const containerRef = useRef<HTMLDivElement>(null)
  const eventSourceRef = useRef<EventSource | null>(null)

  useEffect(() => {
    loadLogs()
    startStream()
    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close()
      }
    }
  }, [])

  useEffect(() => {
    if (autoRefresh && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight
    }
  }, [logs, autoRefresh])

  async function loadLogs() {
    try {
      const res = await fetch('/api/logs')
      const data = await res.json()
      setLogs(data || [])
    } catch (e) {
      console.error('Failed to load logs:', e)
    }
  }

  function startStream() {
    const es = new EventSource('/api/logs/stream')
    es.onmessage = (e) => {
      const data = JSON.parse(e.data)
      setLogs(data || [])
    }
    es.onerror = () => {
      setTimeout(startStream, 2000)
    }
    eventSourceRef.current = es
  }

  async function clearLogs() {
    await fetch('/api/logs/clear', { method: 'POST' })
    setLogs([])
  }

  function getStatusClass(status: string) {
    if (!status) return ''
    if (status.startsWith('2')) return 'text-green-400'
    if (status.startsWith('4')) return 'text-red-400'
    if (status.startsWith('5')) return 'text-red-500'
    return 'text-yellow-400'
  }

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-2xl font-bold flex items-center gap-2">
          <Terminal size={24} />
          Logs
        </h2>
        <div className="flex items-center gap-3">
          <span className="text-sm text-[var(--text-muted)]">
            {logs.length} entries
          </span>
          <button
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`flex items-center gap-1.5 text-sm px-3 py-1.5 rounded border ${
              autoRefresh
                ? 'bg-[var(--accent)] text-white border-[var(--accent)]'
                : 'bg-[var(--bg-card)] text-[var(--text-muted)] border-[var(--border)]'
            }`}
          >
            <RefreshCw size={14} className={autoRefresh ? 'animate-spin' : ''} />
            Auto-scroll
          </button>
          <button
            onClick={clearLogs}
            className="flex items-center gap-1.5 text-sm px-3 py-1.5 rounded border border-[var(--border)] bg-[var(--bg-card)] hover:bg-[var(--bg-hover)] text-[var(--text-muted)]"
          >
            <Trash2 size={14} />
            Clear
          </button>
        </div>
      </div>

      <div ref={containerRef} className="flex-1 overflow-auto bg-[var(--bg-card)] rounded-lg border border-[var(--border)]">
        {logs.length === 0 ? (
          <div className="flex items-center justify-center h-full text-[var(--text-muted)]">
            No logs yet. Send a request to see logs here.
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead className="sticky top-0 bg-[var(--bg-card)] border-b border-[var(--border)]">
              <tr className="text-left text-[var(--text-muted)]">
                <th className="px-3 py-2 font-medium w-24">Time</th>
                <th className="px-3 py-2 font-medium w-20">Provider</th>
                <th className="px-3 py-2 font-medium w-16">Method</th>
                <th className="px-3 py-2 font-medium w-16">Status</th>
                <th className="px-3 py-2 font-medium w-20">Duration</th>
                <th className="px-3 py-2 font-medium w-28">Connection</th>
                <th className="px-3 py-2 font-medium w-28">Model</th>
                <th className="px-3 py-2 font-medium">Message</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--border)]">
              {logs.map((log, i) => {
                const parts = parseLogMessage(log.message)
                return (
                  <tr 
                    key={i} 
                    className={`hover:bg-[var(--bg-hover)] ${log.level === 'ERROR' ? 'bg-red-900/10' : ''}`}
                  >
                    <td className="px-3 py-2 text-[var(--text-muted)] font-mono text-xs">
                      {log.timestamp}
                    </td>
                    <td className="px-3 py-2">
                      <span className={`font-bold ${
                        log.provider === 'KIRO' ? 'text-orange-400' : 'text-green-400'
                      }`}>
                        {log.provider}
                      </span>
                    </td>
                    <td className="px-3 py-2 text-[var(--text-muted)]">
                      {parts.method || '-'}
                    </td>
                    <td className={`px-3 py-2 font-mono font-bold ${getStatusClass(parts.status || '')}`}>
                      {parts.status || '-'}
                    </td>
                    <td className="px-3 py-2 text-[var(--text-muted)] font-mono text-xs">
                      {parts.duration || '-'}
                    </td>
                    <td className="px-3 py-2 text-purple-400 text-xs">
                      {parts.conn || '-'}
                    </td>
                    <td className="px-3 py-2 text-orange-400 text-xs">
                      {parts.model || '-'}
                    </td>
                    <td className="px-3 py-2 text-xs max-w-md truncate" title={log.message}>
                      {log.message}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
