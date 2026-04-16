import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { LiveRequestStream } from './live-request-stream'
import { ConnectionUsageCharts } from './connection-usage-charts'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Activity, AlertTriangle, BarChart3, CheckCircle2 } from 'lucide-react'
import type { LogConnectionSummary } from '@/types/logs'

interface RecentError {
  id: string
  createdAt: string
  connectionId: string | null
  errorMessage: string | null
  responseStatus: number
  provider: string | null
}

interface DashboardTabbedViewProps {
  connSummaries: LogConnectionSummary[]
  recentErrors: RecentError[]
  loading: boolean
}

function formatRelativeTime(dateStr: string | null) {
  if (!dateStr) return 'Never'
  const diffMs = Date.now() - new Date(dateStr).getTime()
  const sec = Math.floor(diffMs / 1000)
  if (sec < 60) return `${sec}s ago`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}h ago`
  return `${Math.floor(hr / 24)}d ago`
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

function ActivityTab({ summaries, loading }: { summaries: LogConnectionSummary[]; loading: boolean }) {
  return (
    <div className="p-4 space-y-4">
      <LiveRequestStream />
      <ConnectionUsageCharts summaries={summaries} loading={loading} />
    </div>
  )
}

function ErrorsTab({ errors, loading }: { errors: RecentError[]; loading: boolean }) {
  if (loading) {
    return (
      <div className="space-y-2 p-4">
        {[1, 2, 3].map(i => (<Skeleton key={i} className="h-14 w-full rounded-lg" />))}
      </div>
    )
  }

  if (errors.length === 0) {
    return (
      <div className="text-center py-10 text-muted-foreground">
        <CheckCircle2 className="size-8 mx-auto mb-2 text-emerald-500" />
        <p className="text-sm">No recent errors</p>
      </div>
    )
  }

  return (
    <div className="space-y-2 p-4 max-h-[400px] overflow-y-auto">
      {errors.map(err => (
        <div
          key={err.id}
          className="flex items-start gap-3 p-2.5 rounded-lg bg-red-50/50 dark:bg-red-950/10"
        >
          <Badge variant="destructive" className="text-[10px] px-1.5 py-0 shrink-0">{err.responseStatus}</Badge>
          <div className="flex-1 min-w-0">
            <p className="text-xs text-red-700 dark:text-red-400 break-words">{err.errorMessage || 'Unknown error'}</p>
            <p className="text-[10px] text-muted-foreground mt-0.5">
              {err.provider?.replace(/_/g, ' ')} · {formatRelativeTime(err.createdAt)}
            </p>
          </div>
        </div>
      ))}
    </div>
  )
}

function AnalyticsTab({ summaries, loading }: { summaries: LogConnectionSummary[]; loading: boolean }) {
  if (loading) {
    return (
      <div className="space-y-3 p-4">
        {[1, 2, 3].map(i => (<Skeleton key={i} className="h-10 w-full rounded-lg" />))}
      </div>
    )
  }

  if (summaries.length === 0) {
    return (
      <div className="text-center py-10 text-muted-foreground text-sm">No usage data yet</div>
    )
  }

  return (
    <div className="p-4 space-y-1">
      {summaries.sort((a, b) => b.requests - a.requests).map(s => {
        const rate = s.requests > 0 ? (((s.requests - s.errors) / s.requests) * 100).toFixed(0) : '-'
        return (
          <div key={s.connectionId} className="flex items-center justify-between py-2 px-2 rounded-md hover:bg-muted/50 text-sm">
            <div className="flex items-center gap-2 min-w-0">
              <span className="font-medium truncate">{s.connectionName || s.connectionId.slice(0, 8)}</span>
              <span className="text-[10px] text-muted-foreground capitalize">{s.provider}</span>
            </div>
            <div className="flex items-center gap-3 shrink-0 text-xs tabular-nums text-muted-foreground">
              <span>{s.requests} req</span>
              <span>{formatTokens(s.totalTokens)} tok</span>
              <span className={s.errors > 0 ? 'text-red-500' : 'text-emerald-500'}>{rate}%</span>
            </div>
          </div>
        )
      })}
    </div>
  )
}

export function DashboardTabbedView({ connSummaries, recentErrors, loading }: DashboardTabbedViewProps) {
  const errorCount = recentErrors?.length || 0

  return (
    <Tabs defaultValue="activity" className="w-full">
      <div className="flex items-center gap-1 border-b px-2">
        <TabsList className="bg-transparent h-auto p-0 gap-0">
          <TabTrigger value="activity" icon={<Activity className="size-3.5" />} label="Activity" />
          <TabTrigger value="errors" icon={<AlertTriangle className="size-3.5" />} label="Errors" badge={errorCount > 0 ? errorCount : undefined} />
          <TabTrigger value="analytics" icon={<BarChart3 className="size-3.5" />} label="Analytics" />
        </TabsList>
      </div>

      <TabsContent value="activity" className="mt-0">
        <ActivityTab summaries={connSummaries} loading={loading} />
      </TabsContent>

      <TabsContent value="errors" className="mt-0">
        <ErrorsTab errors={recentErrors} loading={loading} />
      </TabsContent>

      <TabsContent value="analytics" className="mt-0">
        <AnalyticsTab summaries={connSummaries} loading={loading} />
      </TabsContent>
    </Tabs>
  )
}

function TabTrigger({ value, icon, label, badge }: { value: string; icon: React.ReactNode; label: string; badge?: number }) {
  return (
    <TabsTrigger
      value={value}
      className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none px-3 py-2.5 text-sm gap-1.5"
    >
      {icon}
      {label}
      {badge != null && (
        <Badge variant="destructive" className="text-[9px] px-1 py-0 ml-0.5">{badge}</Badge>
      )}
    </TabsTrigger>
  )
}
