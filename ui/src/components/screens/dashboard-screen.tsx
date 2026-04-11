import { useEffect, useState } from 'react'
import { motion } from 'framer-motion'
import {
  ArrowUpRight,
  ArrowDownRight,
  CheckCircle2,
  Clock,
  Link2,
  Activity,
  AlertTriangle,
} from 'lucide-react'
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  Tooltip as RechartsTooltip,
} from 'recharts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { goApi } from '@/lib/go-api'

interface DashboardData {
  totalRequests: number
  successRequests: number
  errorRequests: number
  avgLatencyMs: number
  activeConnections: number
  totalConnections: number
  activeCombos: number
  requestsByHour: { hour: string; count: number }[]
  requestsByProvider: { provider: string; count: number }[]
  requestsByStatus: { status: number; count: number }[]
  recentErrors: {
    id: string
    createdAt: string
    connectionId: string | null
    errorMessage: string | null
    responseStatus: number
    provider: string | null
  }[]
  connectionHealth: {
    id: string
    name: string
    provider: string
    status: string
    hasModelRestrictions: boolean
    lastError: string | null
  }[]
}

const PROVIDER_COLORS: Record<string, string> = {
  openai: '#10b981',
  anthropic: '#f59e0b',
  kiro: '#ef4444',
  aws_bedrock: '#8b5cf6',
  custom: '#6b7280',
}

const STATUS_CONFIG: Record<string, { label: string; className: string }> = {
  active: { label: 'Active', className: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 border-emerald-200 dark:border-emerald-800' },
  idle: { label: 'Idle', className: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400 border-gray-200 dark:border-gray-700' },
  error: { label: 'Error', className: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400 border-red-200 dark:border-red-800' },
  rate_limited: { label: 'Rate Limited', className: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 border-amber-200 dark:border-amber-800' },
  expired: { label: 'Expired', className: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400 border-yellow-200 dark:border-yellow-800' },
}

function formatHourLabel(hour: string) {
  const parts = hour.split(' ')
  if (parts.length < 2) return hour
  const time = parts[1]
  const [h] = time.split(':')
  const hourNum = parseInt(h, 10)
  const ampm = hourNum >= 12 ? 'PM' : 'AM'
  const displayHour = hourNum % 12 || 12
  return `${displayHour}${ampm}`
}

function formatRelativeTime(dateStr: string | null) {
  if (!dateStr) return 'Never'
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  const diffMin = Math.floor(diffSec / 60)
  const diffHr = Math.floor(diffMin / 60)
  const diffDay = Math.floor(diffHr / 24)

  if (diffSec < 60) return `${diffSec}s ago`
  if (diffMin < 60) return `${diffMin}m ago`
  if (diffHr < 24) return `${diffHr}h ago`
  return `${diffDay}d ago`
}

export default function DashboardScreen() {
  const [data, setData] = useState<DashboardData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    async function fetchDashboard() {
      try {
        setLoading(true)
        const [connections, combos, logsSummary] = await Promise.all([
          goApi.getConnections().catch(() => []),
          goApi.getCombos().catch(() => []),
          goApi.getLogSummary().catch(() => null),
        ])
        
        const activeConnections = connections.filter((c: { status: string }) => c.status === 'active').length
        
        const dashboardData: DashboardData = {
          totalRequests: logsSummary?.totalRequests || 0,
          successRequests: logsSummary?.successRequests || 0,
          errorRequests: logsSummary?.errorRequests || 0,
          avgLatencyMs: logsSummary?.avgLatencyMs || 0,
          activeConnections,
          totalConnections: connections.length,
          activeCombos: combos.length,
          requestsByHour: logsSummary?.requestsByHour || [],
          requestsByProvider: logsSummary?.requestsByProvider || [],
          requestsByStatus: logsSummary?.requestsByStatus || [],
          recentErrors: logsSummary?.recentErrors || [],
          connectionHealth: connections.map((c: { id: string; name: string; provider: string; status: string; supportedModels?: string[]; lastError?: string | null }) => ({
            id: c.id,
            name: c.name,
            provider: c.provider,
            status: c.status,
            hasModelRestrictions: Array.isArray(c.supportedModels) && c.supportedModels.length > 0,
            lastError: c.lastError || null,
          })),
        }
        
        setData(dashboardData)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Unknown error')
      } finally {
        setLoading(false)
      }
    }
    fetchDashboard()
  }, [])

  const successRate = data
    ? data.totalRequests > 0
      ? ((data.successRequests / data.totalRequests) * 100).toFixed(1)
      : '100.0'
    : '0.0'

  const successRateNum = parseFloat(successRate)

  const containerVariants = {
    hidden: { opacity: 0 },
    visible: {
      opacity: 1,
      transition: { staggerChildren: 0.08 },
    },
  }

  const itemVariants = {
    hidden: { opacity: 0, y: 20 },
    visible: { opacity: 1, y: 0, transition: { duration: 0.4 } },
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-2xl font-bold">Dashboard</h1>
          <p className="text-muted-foreground mt-1">Overview of your proxy server performance</p>
        </div>
        <Card className="border-red-200 dark:border-red-800">
          <CardContent className="p-6">
            <div className="flex items-center gap-3 text-red-600 dark:text-red-400">
              <AlertTriangle className="size-5" />
              <span>Failed to load dashboard: {error}</span>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <motion.div
      className="space-y-6"
      variants={containerVariants}
      initial="hidden"
      animate="visible"
    >
      <motion.div variants={itemVariants}>
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <p className="text-muted-foreground mt-1">Overview of your proxy server performance</p>
      </motion.div>

      {/* Stats Cards */}
      <motion.div variants={itemVariants} className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {loading ? (
          Array.from({ length: 4 }).map((_, i) => (
            <Card key={i}>
              <CardContent className="p-6">
                <div className="flex items-center justify-between">
                  <Skeleton className="size-10 rounded-lg" />
                  <Skeleton className="size-5" />
                </div>
                <Skeleton className="h-8 w-24 mt-3" />
                <Skeleton className="h-4 w-32 mt-1" />
              </CardContent>
            </Card>
          ))
        ) : (
          <>
            {/* Total Requests */}
            <Card>
              <CardContent className="p-6">
                <div className="flex items-center justify-between">
                  <div className="size-10 rounded-lg bg-emerald-100 dark:bg-emerald-900/30 flex items-center justify-center">
                    <Activity className="size-5 text-emerald-600 dark:text-emerald-400" />
                  </div>
                  <div className={`flex items-center gap-1 text-xs font-medium ${data!.errorRequests > 0 ? 'text-red-500' : 'text-emerald-600'}`}>
                    {data!.errorRequests > 0 ? (
                      <ArrowUpRight className="size-3" />
                    ) : (
                      <ArrowDownRight className="size-3" />
                    )}
                    {data!.errorRequests} errors
                  </div>
                </div>
                <div className="mt-3 text-2xl font-bold tracking-tight sm:text-3xl">
                  {data!.totalRequests.toLocaleString()}
                </div>
                <p className="text-xs text-muted-foreground mt-1">Total Requests (all time)</p>
              </CardContent>
            </Card>

            {/* Success Rate */}
            <Card>
              <CardContent className="p-6">
                <div className="flex items-center justify-between">
                  <div className="size-10 rounded-lg bg-emerald-100 dark:bg-emerald-900/30 flex items-center justify-center">
                    <CheckCircle2 className="size-5 text-emerald-600 dark:text-emerald-400" />
                  </div>
                  <div className={`flex items-center gap-1 text-xs font-medium ${successRateNum >= 95 ? 'text-emerald-600' : successRateNum >= 80 ? 'text-amber-600' : 'text-red-600'}`}>
                    {successRateNum >= 95 ? (
                      <ArrowUpRight className="size-3" />
                    ) : (
                      <ArrowDownRight className="size-3" />
                    )}
                    {successRateNum >= 95 ? 'Healthy' : 'Needs attention'}
                  </div>
                </div>
                <div className="mt-3 text-2xl font-bold tracking-tight sm:text-3xl">
                  {successRate}%
                </div>
                <p className="text-xs text-muted-foreground mt-1">Success Rate</p>
              </CardContent>
            </Card>

            {/* Avg Latency */}
            <Card>
              <CardContent className="p-6">
                <div className="flex items-center justify-between">
                  <div className="size-10 rounded-lg bg-amber-100 dark:bg-amber-900/30 flex items-center justify-center">
                    <Clock className="size-5 text-amber-600 dark:text-amber-400" />
                  </div>
                  <div className={`flex items-center gap-1 text-xs font-medium ${data!.avgLatencyMs < 500 ? 'text-emerald-600' : data!.avgLatencyMs < 1000 ? 'text-amber-600' : 'text-red-600'}`}>
                    <ArrowDownRight className="size-3" />
                    {data!.avgLatencyMs < 500 ? 'Fast' : data!.avgLatencyMs < 1000 ? 'Moderate' : 'Slow'}
                  </div>
                </div>
                <div className="mt-3 text-2xl font-bold tracking-tight sm:text-3xl">
                  {Math.round(data!.avgLatencyMs)}ms
                </div>
                <p className="text-xs text-muted-foreground mt-1">Avg Latency</p>
              </CardContent>
            </Card>

            {/* Active Connections */}
            <Card>
              <CardContent className="p-6">
                <div className="flex items-center justify-between">
                  <div className="size-10 rounded-lg bg-violet-100 dark:bg-violet-900/30 flex items-center justify-center">
                    <Link2 className="size-5 text-violet-600 dark:text-violet-400" />
                  </div>
                  <div className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
                    {data!.totalConnections} total
                  </div>
                </div>
                <div className="mt-3 text-2xl font-bold tracking-tight sm:text-3xl">
                  {data!.activeConnections}
                </div>
                <p className="text-xs text-muted-foreground mt-1">Active Connections</p>
              </CardContent>
            </Card>
          </>
        )}
      </motion.div>

      {/* Charts Section */}
      <motion.div variants={itemVariants} className="grid grid-cols-1 lg:grid-cols-5 gap-4">
        {/* Bar Chart */}
        <Card className="lg:col-span-3">
          <CardHeader className="pb-2">
            <CardTitle className="text-base font-semibold">Requests by Hour (Last 24h)</CardTitle>
          </CardHeader>
          <CardContent className="p-4 pt-0">
            {loading ? (
              <Skeleton className="h-[280px] w-full" />
            ) : (
              <ResponsiveContainer width="100%" height={280}>
                <BarChart data={data?.requestsByHour} margin={{ top: 5, right: 10, left: -20, bottom: 5 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                  <XAxis
                    dataKey="hour"
                    tickFormatter={formatHourLabel}
                    tick={{ fontSize: 11, fill: 'hsl(var(--muted-foreground))' }}
                    interval={2}
                  />
                  <YAxis tick={{ fontSize: 11, fill: 'hsl(var(--muted-foreground))' }} />
                  <RechartsTooltip
                    labelFormatter={formatHourLabel}
                    contentStyle={{
                      backgroundColor: 'hsl(var(--popover))',
                      borderColor: 'hsl(var(--border))',
                      borderRadius: '8px',
                      fontSize: '12px',
                    }}
                  />
                  <defs>
                    <linearGradient id="barGradient" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="#10b981" stopOpacity={0.9} />
                      <stop offset="100%" stopColor="#10b981" stopOpacity={0.3} />
                    </linearGradient>
                  </defs>
                  <Bar dataKey="count" fill="url(#barGradient)" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        {/* Pie Chart */}
        <Card className="lg:col-span-2">
          <CardHeader className="pb-2">
            <CardTitle className="text-base font-semibold">Requests by Provider</CardTitle>
          </CardHeader>
          <CardContent className="p-4 pt-0">
            {loading ? (
              <Skeleton className="h-[280px] w-full" />
            ) : data?.requestsByProvider && data.requestsByProvider.length > 0 ? (
              <div>
                <ResponsiveContainer width="100%" height={200}>
                  <PieChart>
                    <Pie
                      data={data.requestsByProvider}
                      cx="50%"
                      cy="50%"
                      innerRadius={50}
                      outerRadius={80}
                      paddingAngle={3}
                      dataKey="count"
                      nameKey="provider"
                    >
                      {data.requestsByProvider.map((entry, index) => (
                        <Cell
                          key={index}
                          fill={PROVIDER_COLORS[entry.provider] || PROVIDER_COLORS.custom}
                        />
                      ))}
                    </Pie>
                    <RechartsTooltip
                      contentStyle={{
                        backgroundColor: 'hsl(var(--popover))',
                        borderColor: 'hsl(var(--border))',
                        borderRadius: '8px',
                        fontSize: '12px',
                      }}
                    />
                  </PieChart>
                </ResponsiveContainer>
                <div className="flex flex-wrap justify-center gap-3 mt-2">
                  {data.requestsByProvider.map((entry) => (
                    <div key={entry.provider} className="flex items-center gap-1.5 text-xs">
                      <div
                        className="size-2.5 rounded-full"
                        style={{ backgroundColor: PROVIDER_COLORS[entry.provider] || PROVIDER_COLORS.custom }}
                      />
                      <span className="text-muted-foreground capitalize">
                        {entry.provider.replace(/_/g, ' ')}
                      </span>
                      <span className="font-medium">{entry.count}</span>
                    </div>
                  ))}
                </div>
              </div>
            ) : (
              <div className="h-[280px] flex items-center justify-center text-muted-foreground text-sm">
                No request data available
              </div>
            )}
          </CardContent>
        </Card>
      </motion.div>

      {/* Connection Health Table */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-semibold">Connection Health</CardTitle>
          </CardHeader>
          <CardContent className="p-4 pt-0">
            {loading ? (
              <div className="space-y-3">
                {Array.from({ length: 3 }).map((_, i) => (
                  <Skeleton key={i} className="h-12 w-full" />
                ))}
              </div>
            ) : data?.connectionHealth && data.connectionHealth.length > 0 ? (
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Provider</TableHead>
                      <TableHead>Status</TableHead>
                          <TableHead>Models</TableHead>
                          <TableHead>Last Error</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {data.connectionHealth.map((conn) => {
                      const statusCfg = STATUS_CONFIG[conn.status] || STATUS_CONFIG.idle

                      return (
                        <TableRow key={conn.id}>
                          <TableCell className="font-medium">{conn.name}</TableCell>
                          <TableCell>
                            <span className="capitalize text-muted-foreground text-sm">
                              {conn.provider.replace(/_/g, ' ')}
                            </span>
                          </TableCell>
                          <TableCell>
                            <Badge variant="outline" className={statusCfg.className}>
                              {statusCfg.label}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-muted-foreground text-sm">
                            {conn.hasModelRestrictions ? 'Restricted list' : 'All supported models'}
                          </TableCell>
                          <TableCell className="text-muted-foreground text-sm">
                            {conn.lastError || 'None'}
                          </TableCell>
                        </TableRow>
                      )
                    })}
                  </TableBody>
                </Table>
              </div>
            ) : (
              <div className="text-center py-8 text-muted-foreground text-sm">
                No connections configured yet
              </div>
            )}
          </CardContent>
        </Card>
      </motion.div>

      {/* Recent Errors */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-semibold flex items-center gap-2">
              <AlertTriangle className="size-4 text-red-500" />
              Recent Errors
            </CardTitle>
          </CardHeader>
          <CardContent className="p-4 pt-0">
            {loading ? (
              <div className="space-y-3">
                {Array.from({ length: 3 }).map((_, i) => (
                  <Skeleton key={i} className="h-16 w-full" />
                ))}
              </div>
            ) : data?.recentErrors && data.recentErrors.length > 0 ? (
              <div className="space-y-3 max-h-96 overflow-y-auto">
                {data.recentErrors.map((err) => (
                  <div
                    key={err.id}
                    className="flex flex-col sm:flex-row sm:items-start gap-2 sm:gap-4 p-3 rounded-lg bg-red-50 dark:bg-red-950/20 border border-red-100 dark:border-red-900/30"
                  >
                    <div className="flex items-center gap-2 shrink-0">
                      <Badge variant="destructive" className="text-xs">
                        {err.responseStatus}
                      </Badge>
                      <span className="text-xs text-muted-foreground">
                        {formatRelativeTime(err.createdAt)}
                      </span>
                    </div>
                    <div className="flex-1 min-w-0">
                      {err.provider && (
                        <p className="text-xs font-medium text-muted-foreground mb-0.5 capitalize">
                          {err.provider.replace(/_/g, ' ')}
                        </p>
                      )}
                      <p className="text-sm text-red-700 dark:text-red-400 break-words">
                        {err.errorMessage || 'Unknown error'}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-8 text-muted-foreground text-sm">
                <CheckCircle2 className="size-8 mx-auto mb-2 text-emerald-500" />
                No recent errors — everything is running smoothly!
              </div>
            )}
          </CardContent>
        </Card>
      </motion.div>
    </motion.div>
  )
}
