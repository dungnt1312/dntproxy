import { useEffect, useState, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { DashboardHeader } from '@/components/dashboard/dashboard-header'
import { StatPills } from '@/components/dashboard/stat-pills'
import { DashboardTabbedView } from '@/components/dashboard/dashboard-tabbed-view'
import { Card } from '@/components/ui/card'
import { goApi } from '@/lib/go-api'
import type { LogConnectionSummary } from '@/types/logs'

interface DashboardData {
  totalRequests: number
  errorRequests: number
  avgLatencyMs: number
  costTotal: number
  recentErrors: {
    id: string
    createdAt: string
    connectionId: string | null
    errorMessage: string | null
    responseStatus: number
    provider: string | null
  }[]
}

export default function DashboardScreen() {
  const navigate = useNavigate()
  const [data, setData] = useState<DashboardData | null>(null)
  const [connSummaries, setConnSummaries] = useState<LogConnectionSummary[]>([])
  const [loading, setLoading] = useState(true)
  const intervalRef = useRef<ReturnType<typeof setInterval>>(undefined)

  const fetchData = useCallback(async () => {
    try {
      const [, , logsSummary, logConns] = await Promise.all([
        goApi.getConnections().catch(() => []),
        goApi.getCombos().catch(() => []),
        goApi.getLogSummary().catch(() => null),
        goApi.getLogConnections({ range: '24h' }).catch((): LogConnectionSummary[] => []),
      ])

      setConnSummaries(Array.isArray(logConns) ? logConns : [])

      setData({
        totalRequests: logsSummary?.totalRequests || 0,
        errorRequests: logsSummary?.errorRequests || 0,
        avgLatencyMs: logsSummary?.avgLatencyMs || 0,
        costTotal: logsSummary?.totalCost || 0,
        recentErrors: logsSummary?.recentErrors || [],
      })
    } catch {
      // silent
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
    intervalRef.current = setInterval(fetchData, 30_000)
    return () => { if (intervalRef.current) clearInterval(intervalRef.current) }
  }, [fetchData])

  const successRate = data
    ? data.totalRequests > 0
      ? ((data.totalRequests - data.errorRequests) / data.totalRequests) * 100
      : 100
    : 0

  return (
    <div className="space-y-4">
      <DashboardHeader navigate={navigate} />

      <StatPills
        totalRequests={data?.totalRequests || 0}
        avgLatencyMs={data?.avgLatencyMs || 0}
        costTotal={data?.costTotal || 0}
        successRate={successRate}
        loading={loading}
      />

      <Card>
        <DashboardTabbedView
          connSummaries={connSummaries}
          recentErrors={data?.recentErrors || []}
          loading={loading}
        />
      </Card>
    </div>
  )
}
