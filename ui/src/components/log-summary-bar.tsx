import { Activity, DollarSign, TriangleAlert, Zap } from 'lucide-react'
import type { LogSummary } from '../types/logs'

interface LogSummaryBarProps {
  summary: LogSummary | null
}

function formatNumber(value = 0) {
  return new Intl.NumberFormat().format(value)
}

function formatCost(value = 0, currency = 'USD') {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency,
    maximumFractionDigits: 4,
  }).format(value)
}

const statConfig = [
  { label: 'Requests', icon: Activity, colorClass: 'stat-card-blue' },
  { label: 'Errors', icon: TriangleAlert, colorClass: 'stat-card-danger' },
  { label: 'Tokens', icon: Zap, colorClass: 'stat-card-green' },
  { label: 'Est. Cost', icon: DollarSign, colorClass: 'stat-card-amber' },
]

export default function LogSummaryBar({ summary }: LogSummaryBarProps) {
  const values = [
    formatNumber(summary?.requests),
    formatNumber(summary?.errors),
    formatNumber(summary?.totalTokens),
    formatCost(summary?.costTotal, summary?.currency || 'USD'),
  ]

  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
      {statConfig.map((item, i) => (
        <div key={item.label} className={`glass stat-card ${item.colorClass} cursor-default`}>
          <div className="flex items-center gap-2 text-[var(--text-dim)] text-[10px] uppercase font-bold tracking-wider mb-1">
            <item.icon size={12} />
            {item.label}
          </div>
          <div className="text-xl font-bold" style={{ fontFamily: 'var(--font-heading)' }}>{values[i]}</div>
        </div>
      ))}
    </div>
  )
}
