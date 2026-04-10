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
    <aside className="lg:w-72 glass overflow-hidden">
      <button
        onClick={() => onSelect('all')}
        className={`w-full text-left px-4 py-3 border-b border-[var(--border)] transition-colors cursor-pointer ${selectedId === 'all' ? 'bg-[var(--accent-glow)]' : 'hover:bg-white/[0.02]'}`}
      >
        <div className="font-medium text-sm" style={{ fontFamily: 'var(--font-heading)' }}>All connections</div>
        <div className="text-[10px] text-[var(--text-dim)]">{safeConnections.length} active in logs</div>
      </button>
      <div className="max-h-[560px] overflow-auto">
        {safeConnections.length === 0 ? (
          <div className="p-4 text-xs text-[var(--text-dim)]">No connection usage in selected range.</div>
        ) : safeConnections.map(conn => (
          <button
            key={conn.connectionId}
            onClick={() => onSelect(conn.connectionId)}
            className={`w-full text-left px-4 py-3 border-b border-[var(--border)] transition-colors cursor-pointer ${selectedId === conn.connectionId ? 'bg-[var(--accent-glow)]' : 'hover:bg-white/[0.02]'}`}
          >
            <div className="flex items-center justify-between gap-2">
              <span className="font-medium text-sm truncate">{conn.connectionName || conn.connectionId}</span>
              {conn.errors > 0 && <span className="chip chip-danger text-[10px]">{conn.errors} err</span>}
            </div>
            <div className="mt-0.5 text-[10px] text-[var(--text-dim)] truncate">{conn.provider}</div>
            <div className="mt-2 grid grid-cols-2 gap-2 text-[11px] text-[var(--text-muted)]">
              <span className="font-mono">{conn.totalTokens.toLocaleString()} tok</span>
              <span className="text-right font-mono">{formatCost(conn.costTotal, conn.currency || 'USD')}</span>
            </div>
          </button>
        ))}
      </div>
    </aside>
  )
}
