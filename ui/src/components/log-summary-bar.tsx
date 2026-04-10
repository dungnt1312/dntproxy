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

export default function LogSummaryBar({ summary }: LogSummaryBarProps) {
  const items = [
    { label: 'Requests', value: formatNumber(summary?.requests), icon: Activity },
    { label: 'Errors', value: formatNumber(summary?.errors), icon: TriangleAlert },
    { label: 'Tokens', value: formatNumber(summary?.totalTokens), icon: Zap },
    { label: 'Estimated cost', value: formatCost(summary?.costTotal, summary?.currency || 'USD'), icon: DollarSign },
  ]

  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
      {items.map(item => (
        <div key={item.label} className="border border-[var(--border)] bg-[var(--bg-card)] rounded-lg p-3">
          <div className="flex items-center gap-2 text-[var(--text-muted)] text-xs uppercase">
            <item.icon size={14} />
            {item.label}
          </div>
          <div className="mt-2 text-xl font-semibold">{item.value}</div>
        </div>
      ))}
    </div>
  )
}
