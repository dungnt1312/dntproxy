import type { LogConnectionSummary } from '../types/logs'

interface LogConnectionListProps {
  connections: LogConnectionSummary[]
  selectedId: string
  onSelect: (connectionId: string) => void
}

function formatCost(value = 0, currency = 'USD') {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency,
    maximumFractionDigits: 4,
  }).format(value)
}

export default function LogConnectionList({ connections, selectedId, onSelect }: LogConnectionListProps) {
  const safeConnections = Array.isArray(connections) ? connections : []

  return (
    <aside className="lg:w-72 border border-[var(--border)] bg-[var(--bg-card)] rounded-lg overflow-hidden">
      <button
        onClick={() => onSelect('all')}
        className={`w-full text-left px-3 py-3 border-b border-[var(--border)] ${selectedId === 'all' ? 'bg-[var(--bg-hover)]' : ''}`}
      >
        <div className="font-medium">All connections</div>
        <div className="text-xs text-[var(--text-muted)]">{safeConnections.length} active in logs</div>
      </button>
      <div className="max-h-[560px] overflow-auto">
        {safeConnections.length === 0 ? (
          <div className="p-4 text-sm text-[var(--text-muted)]">No connection usage in selected range.</div>
        ) : safeConnections.map(conn => (
          <button
            key={conn.connectionId}
            onClick={() => onSelect(conn.connectionId)}
            className={`w-full text-left px-3 py-3 border-b border-[var(--border)] hover:bg-[var(--bg-hover)] ${selectedId === conn.connectionId ? 'bg-[var(--bg-hover)]' : ''}`}
          >
            <div className="flex items-center justify-between gap-2">
              <span className="font-medium truncate">{conn.connectionName || conn.connectionId}</span>
              {conn.errors > 0 && <span className="text-xs text-[var(--danger)]">{conn.errors} err</span>}
            </div>
            <div className="mt-1 text-xs text-[var(--text-muted)] truncate">{conn.provider}</div>
            <div className="mt-2 grid grid-cols-2 gap-2 text-xs">
              <span>{conn.totalTokens.toLocaleString()} tokens</span>
              <span className="text-right">{formatCost(conn.costTotal, conn.currency || 'USD')}</span>
            </div>
          </button>
        ))}
      </div>
    </aside>
  )
}
