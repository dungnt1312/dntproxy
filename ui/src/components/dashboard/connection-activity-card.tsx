import { ProviderLogo } from '@/components/connections/ProviderLogo'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Activity, CheckCircle2, Clock, Coins, Zap, AlertTriangle } from 'lucide-react'
import { cn } from '@/lib/utils'
import { formatTokens } from '@/components/screens/dashboard/helpers'
import type { LogConnectionSummary } from '@/types/logs'

export type ConnectionStatus = 'active' | 'recent' | 'idle' | 'error' | 'rate_limited'

interface ConnectionActivityCardProps {
  id: string
  name: string
  provider: string
  status: ConnectionStatus
  summary: LogConnectionSummary
  rateLimitedUntil?: string | null
  lastError?: string | null
  onClick?: () => void
}

const STATUS_CONFIG: Record<ConnectionStatus, { dotClass: string; label: string; badgeClass: string; pulse: boolean }> = {
  active: {
    dotClass: 'bg-emerald-500',
    label: 'Active',
    badgeClass: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 border-emerald-200 dark:border-emerald-800',
    pulse: true,
  },
  recent: {
    dotClass: 'bg-amber-500',
    label: 'Recent',
    badgeClass: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 border-amber-200 dark:border-amber-800',
    pulse: false,
  },
  idle: {
    dotClass: 'bg-gray-400 dark:bg-gray-600',
    label: 'Idle',
    badgeClass: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400 border-gray-200 dark:border-gray-700',
    pulse: false,
  },
  error: {
    dotClass: 'bg-red-500',
    label: 'Error',
    badgeClass: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400 border-red-200 dark:border-red-800',
    pulse: false,
  },
  rate_limited: {
    dotClass: 'bg-orange-500',
    label: 'Rate Limited',
    badgeClass: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400 border-orange-200 dark:border-orange-800',
    pulse: false,
  },
}

function formatRelativeTimestamp(ms: number): string {
  if (!ms) return 'Never'
  const diff = Date.now() - ms
  if (diff < 0) return 'Just now'
  const sec = Math.floor(diff / 1000)
  if (sec < 5) return 'Just now'
  if (sec < 60) return `${sec}s ago`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}h ago`
  return `${Math.floor(hr / 24)}d ago`
}

function formatCooldown(until: string | null | undefined): string {
  if (!until) return ''
  const diff = new Date(until).getTime() - Date.now()
  if (diff <= 0) return ''
  const sec = Math.ceil(diff / 1000)
  if (sec < 60) return `${sec}s`
  return `${Math.floor(sec / 60)}m ${sec % 60}s`
}

export function ConnectionActivityCard({
  name,
  provider,
  status,
  summary,
  rateLimitedUntil,
  lastError,
  onClick,
}: ConnectionActivityCardProps) {
  const cfg = STATUS_CONFIG[status]
  const successRate = summary.requests > 0
    ? (((summary.requests - summary.errors) / summary.requests) * 100).toFixed(0)
    : '-'
  const successRateNum = successRate !== '-' ? parseFloat(successRate) : -1
  const cooldown = formatCooldown(rateLimitedUntil)

  return (
    <Card
      className={cn(
        'cursor-pointer transition-all duration-200 hover:shadow-md hover:-translate-y-0.5 border',
        status === 'active' && 'border-emerald-200 dark:border-emerald-800/50',
        status === 'error' && 'border-red-200 dark:border-red-800/50',
        status === 'rate_limited' && 'border-orange-200 dark:border-orange-800/50',
      )}
      onClick={onClick}
    >
      <CardContent className="p-4 space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 min-w-0">
            <div className="relative shrink-0">
              <ProviderLogo provider={provider} size={20} />
              <div
                className={cn(
                  'absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full border-2 border-white dark:border-gray-950',
                  cfg.dotClass,
                  cfg.pulse && 'animate-pulse',
                )}
              />
            </div>
            <span className="font-semibold text-sm truncate">{name}</span>
          </div>
          <Badge variant="outline" className={cn('text-[10px] px-1.5 py-0 shrink-0', cfg.badgeClass)}>
            {cfg.label}
          </Badge>
        </div>

        <div className="grid grid-cols-2 gap-x-3 gap-y-1.5 text-xs">
          <MetricRow
            icon={<Activity className="size-3" />}
            value={summary.requests > 0 ? String(summary.requests) : '-'}
            label="requests"
          />
          <MetricRow
            icon={<Clock className="size-3" />}
            value={formatRelativeTimestamp(summary.lastUsedMs)}
            label="last used"
          />
          <MetricRow
            icon={<CheckCircle2 className="size-3" />}
            value={successRate === '-' ? '-' : `${successRate}%`}
            label="success"
            valueClass={
              successRateNum >= 95 ? 'text-emerald-600 dark:text-emerald-400' :
              successRateNum >= 80 ? 'text-amber-600 dark:text-amber-400' :
              successRateNum >= 0 ? 'text-red-600 dark:text-red-400' : ''
            }
          />
          <MetricRow
            icon={<Zap className="size-3" />}
            value={summary.totalTokens > 0 ? formatTokens(summary.totalTokens) : '-'}
            label="tokens"
          />
          {summary.costTotal > 0 && (
            <MetricRow
              icon={<Coins className="size-3" />}
              value={`$${summary.costTotal < 0.01 ? summary.costTotal.toFixed(4) : summary.costTotal.toFixed(2)}`}
              label="cost"
            />
          )}
        </div>

        {status === 'rate_limited' && cooldown && (
          <div className="flex items-center gap-1.5 text-[11px] text-orange-600 dark:text-orange-400 pt-1 border-t border-orange-100 dark:border-orange-900/30">
            <AlertTriangle className="size-3" />
            <span>Cooldown: {cooldown}</span>
          </div>
        )}
        {status === 'error' && lastError && (
          <div className="flex items-center gap-1.5 text-[11px] text-red-600 dark:text-red-400 pt-1 border-t border-red-100 dark:border-red-900/30">
            <AlertTriangle className="size-3 shrink-0" />
            <span className="truncate">{lastError}</span>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function MetricRow({ icon, value, label, valueClass }: {
  icon: React.ReactNode
  value: string
  label: string
  valueClass?: string
}) {
  return (
    <div className="flex items-center gap-1.5">
      <span className="text-muted-foreground shrink-0">{icon}</span>
      <span className={cn('font-medium', valueClass)}>{value}</span>
      {value !== '-' && <span className="text-muted-foreground">{label}</span>}
    </div>
  )
}
