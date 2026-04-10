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

const selectClass = 'bg-[var(--bg-card)] border border-[var(--border)] rounded-lg px-3 py-2 text-sm outline-none'

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
    <div className="flex flex-col lg:flex-row gap-3">
      <select className={selectClass} value={filters.range} onChange={e => setFilter('range', e.target.value)}>
        <option value="1h">Last hour</option>
        <option value="24h">Last 24h</option>
        <option value="7d">Last 7d</option>
        <option value="30d">Last 30d</option>
      </select>
      <select className={selectClass} value={filters.provider} onChange={e => setFilter('provider', e.target.value)}>
        <option value="all">All providers</option>
        <option value="CLIENT">Client</option>
        <option value="KIRO">Kiro</option>
        <option value="OPENAI">OpenAI</option>
      </select>
      <select className={selectClass} value={filters.level} onChange={e => setFilter('level', e.target.value)}>
        <option value="all">All levels</option>
        <option value="INFO">Info</option>
        <option value="ERROR">Errors</option>
      </select>
      <div className="relative flex-1 min-w-0">
        <Search size={15} className="absolute left-3 top-2.5 text-[var(--text-muted)]" />
        <input
          value={filters.q}
          onChange={e => setFilter('q', e.target.value)}
          placeholder="Search payload, model, request id, message, error"
          className="w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-lg pl-9 pr-3 py-2 text-sm outline-none"
        />
      </div>
      <button
        onClick={() => onLiveChange(!live)}
        className={`flex items-center justify-center gap-2 px-3 py-2 rounded-lg border text-sm ${
          live ? 'bg-[var(--accent)] border-[var(--accent)] text-white' : 'bg-[var(--bg-card)] border-[var(--border)]'
        }`}
      >
        <RefreshCw size={14} className={live ? 'animate-spin' : ''} />
        Live
      </button>
      <button onClick={onRefresh} className="flex items-center justify-center gap-2 px-3 py-2 rounded-lg border border-[var(--border)] bg-[var(--bg-card)] text-sm">
        <RefreshCw size={14} />
        Refresh
      </button>
      <button onClick={onClear} className="flex items-center justify-center gap-2 px-3 py-2 rounded-lg border border-[var(--border)] bg-[var(--bg-card)] text-sm text-[var(--danger)]">
        <Trash2 size={14} />
        Clear
      </button>
    </div>
  )
}
