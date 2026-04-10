import { RefreshCw, Search, Trash2 } from 'lucide-react'
import type { LogFilters } from '../types/logs'

interface LogFilterBarProps {
  filters: LogFilters
  live: boolean
  onFiltersChange: (filters: LogFilters) => void
  onLiveChange: (live: boolean) => void
  onRefresh: () => void
  onClear: () => void
}

export default function LogFilterBar({
  filters,
  live,
  onFiltersChange,
  onLiveChange,
  onRefresh,
  onClear,
}: LogFilterBarProps) {
  const setFilter = (key: keyof LogFilters, value: string) => {
    onFiltersChange({ ...filters, [key]: value })
  }

  return (
    <div className="glass-sm p-3">
      <div className="flex flex-col lg:flex-row gap-2">
        <select className="glass-select" value={filters.range} onChange={e => setFilter('range', e.target.value)}>
          <option value="1h">Last hour</option>
          <option value="24h">Last 24h</option>
          <option value="7d">Last 7d</option>
          <option value="30d">Last 30d</option>
        </select>
        <select className="glass-select" value={filters.provider} onChange={e => setFilter('provider', e.target.value)}>
          <option value="all">All providers</option>
          <option value="CLIENT">Client</option>
          <option value="KIRO">Kiro</option>
          <option value="OPENAI">OpenAI</option>
        </select>
        <select className="glass-select" value={filters.level} onChange={e => setFilter('level', e.target.value)}>
          <option value="all">All levels</option>
          <option value="INFO">Info</option>
          <option value="ERROR">Errors</option>
        </select>
        <div className="relative flex-1 min-w-0">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-dim)]" />
          <input
            value={filters.q}
            onChange={e => setFilter('q', e.target.value)}
            placeholder="Search payload, model, request id…"
            className="glass-input w-full pl-9 text-xs"
          />
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <button
            onClick={() => onLiveChange(!live)}
            className={`btn-ghost text-xs ${live ? '!bg-[var(--accent)] !text-white !border-[var(--accent)]' : ''}`}
          >
            <RefreshCw size={13} className={live ? 'animate-spin' : ''} />
            Live
          </button>
          <button onClick={onRefresh} className="btn-ghost text-xs">
            <RefreshCw size={13} />
          </button>
          <button onClick={onClear} className="btn-ghost text-xs text-[var(--danger)]">
            <Trash2 size={13} />
          </button>
        </div>
      </div>
    </div>
  )
}
